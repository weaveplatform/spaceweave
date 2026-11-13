// Package trace runs a Claude agent that traces a floor-plan image into a
// plan.Trace. It automates the loop used to trace the microsite's plans by
// hand: look at crops with pixel rulers, measure ink runs, submit a trace,
// read the validator's errors and the trace drawn back over the image, and
// repeat until it lines up.
package trace

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"math"
	"strings"

	"github.com/anthropics/anthropic-sdk-go/v2"

	"github.com/weaveplatform/spaceweave/internal/config"
	"github.com/weaveplatform/spaceweave/internal/plan"
	"github.com/weaveplatform/spaceweave/internal/raster"
)

//go:embed prompt.md
var prompt string

// Errors.
var (
	// ErrNoTrace means the run ended without a valid trace to publish.
	ErrNoTrace = errors.New("tracer stopped without a valid trace")
	// ErrModel is an API failure the SDK's retries didn't recover from.
	ErrModel = errors.New("model request failed")
)

// Reasons a run stops early. A run that stops early with a valid trace still
// publishes it; these go in its caveats.
const (
	StopTurns    = "turn limit reached"
	StopSubmits  = "submission limit reached"
	StopSpend    = "spending cap reached"
	StopTimeout  = "time limit reached"
	StopRefusal  = "the model declined"
	StopContext  = "context window full"
	StopStalled  = "the model stopped using its tools"
	maxNudges    = 2
	maxTokens    = 64000
	taskBudget   = 500_000
	keepToolUses = 8
)

// Options configure a run.
type Options struct {
	Client    anthropic.Client
	Model     string
	Effort    string
	Limits    config.Limits
	Prices    config.Prices
	Validator plan.Validator
	// Transcript, when set, receives one JSON line per turn.
	Transcript io.Writer
}

// Job is one image to trace.
type Job struct {
	Image    *raster.Source
	Title    string    // a first guess, from the file name
	Meta     plan.Meta // carried into plan.json's SOURCE
	Feedback string    // a reviewer's note on the previous attempt
	Previous *plan.Trace
}

// Usage is the run's token count.
type Usage struct {
	Input      int64 `json:"input"`
	Output     int64 `json:"output"`
	CacheRead  int64 `json:"cacheRead"`
	CacheWrite int64 `json:"cacheWrite"`
}

// Result is what a run produced. It is returned even with an error, so the
// caller can report the cost and the reason.
type Result struct {
	Trace      *plan.Trace `json:"-"`
	Plan       *plan.Plan  `json:"-"`
	PlanJSON   []byte      `json:"-"`
	Overlay    []byte      `json:"-"` // PNG at source size
	Confidence float64     `json:"confidence"`
	Summary    string      `json:"summary"`
	Caveats    []string    `json:"caveats"`
	Stopped    string      `json:"stopped,omitempty"`
	Turns      int         `json:"turns"`
	Submits    int         `json:"submits"`
	Usage      Usage       `json:"usage"`
	CostUSD    float64     `json:"costUsd"`
	Problems   []string    `json:"problems,omitempty"` // of the last submission, when it wasn't valid
}

// submission is one submit_trace and its verdict.
type submission struct {
	trace    *plan.Trace
	plan     *plan.Plan
	planJSON []byte
	overlay  *image.RGBA
	problems []string
}

func (s *submission) valid() bool { return s != nil && s.plan != nil && len(s.problems) == 0 }

type run struct {
	opts     Options
	job      Job
	result   Result
	last     *submission
	lastGood *submission
	finished bool
	fatal    error // a failure no retry by the model can fix
}

// Run traces one image.
func Run(ctx context.Context, opts Options, job Job) (*Result, error) {
	r := &run{opts: opts, job: job}
	err := r.loop(ctx)
	return r.finish(err)
}

func (r *run) loop(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, r.opts.Limits.Timeout)
	defer cancel()

	first, err := r.opening()
	if err != nil {
		return err
	}
	messages := []anthropic.BetaMessageParam{anthropic.NewBetaUserMessage(first...)}
	nudges := 0
	for {
		if r.result.Turns >= r.opts.Limits.MaxTurns {
			r.result.Stopped = StopTurns
			return nil
		}
		r.result.Turns++
		msg, err := r.call(ctx, messages)
		if err != nil {
			if ctx.Err() != nil {
				r.result.Stopped = StopTimeout
				return nil
			}
			return err
		}
		messages = append(messages, msg.ToParam())
		r.account(msg.Usage)

		switch msg.StopReason {
		case anthropic.BetaStopReasonRefusal:
			r.result.Stopped = fmt.Sprintf("%s (%s)", StopRefusal, msg.StopDetails.Category)
			return nil
		case anthropic.BetaStopReasonModelContextWindowExceeded:
			r.result.Stopped = StopContext
			return nil
		}

		// Every tool call gets a result, whatever the stop reason, or the next
		// request is rejected.
		reply := r.tools(ctx, msg)
		if r.fatal != nil {
			r.log(msg, reply)
			if ctx.Err() != nil {
				// The validator failed because time ran out, not because it's broken.
				r.result.Stopped = StopTimeout
				return nil
			}
			return r.fatal
		}
		if r.finished {
			r.log(msg, reply)
			return nil
		}
		hasTools := len(reply) > 0
		switch {
		case msg.StopReason == anthropic.BetaStopReasonMaxTokens:
			reply = append(
				reply,
				anthropic.NewBetaTextBlock("Your reply hit the output limit and was cut off. "+
					"If you were writing a trace, submit it again without indentation or commentary."),
			)
		case !hasTools:
			nudges++
			if nudges > maxNudges {
				r.result.Stopped = StopStalled
				r.log(msg, reply)
				return nil
			}
			reply = append(
				reply,
				anthropic.NewBetaTextBlock(
					"Keep working with the tools. When your latest submission "+
						"has no problems and the overlay lines up, call finish.",
				),
			)
		default:
			nudges = 0
		}
		if r.result.Submits >= r.opts.Limits.MaxSubmits && !r.finished {
			if r.last.valid() {
				reply = append(
					reply,
					anthropic.NewBetaTextBlock("That was your last submission. Call finish now."),
				)
			} else {
				r.result.Stopped = StopSubmits
				r.log(msg, reply)
				return nil
			}
		}
		if r.result.CostUSD >= r.opts.Limits.MaxUSD {
			r.result.Stopped = StopSpend
			r.log(msg, reply)
			return nil
		}
		r.log(msg, reply)
		messages = append(messages, anthropic.NewBetaUserMessage(reply...))
	}
}

// opening is the first user turn: the task, any reviewer feedback, and the
// whole image.
func (r *run) opening() ([]anthropic.BetaContentBlockParamUnion, error) {
	w, h := r.job.Image.Size()
	view, err := raster.Crop(r.job.Image.Image, r.job.Image.Image.Bounds(), 0)
	if err != nil {
		return nil, fmt.Errorf("view source: %w", err)
	}
	png, err := raster.EncodePNG(view.Image)
	if err != nil {
		return nil, fmt.Errorf("encode view: %w", err)
	}
	var b strings.Builder
	fmt.Fprintf(
		&b,
		"Trace this floor plan. The image is %d × %d pixels; its file name suggests the title %q.\n",
		w,
		h,
		r.job.Title,
	)
	if r.job.Feedback != "" {
		fmt.Fprintf(
			&b,
			"\nA reviewer sent the previous attempt back with this note. Address it:\n%s\n",
			r.job.Feedback,
		)
	}
	if r.job.Previous != nil {
		prev, err := json.Marshal(r.job.Previous)
		if err != nil {
			return nil, fmt.Errorf("encode previous trace: %w", err)
		}
		fmt.Fprintf(
			&b,
			"\nThe previous attempt's trace, to correct rather than start again:\n%s\n",
			prev,
		)
	}
	fmt.Fprintf(&b, "\nThe whole image, shown at %.2f×:", view.Scale)
	return []anthropic.BetaContentBlockParamUnion{
		anthropic.NewBetaTextBlock(b.String()),
		imageBlock(png),
	}, nil
}

func imageBlock(png []byte) anthropic.BetaContentBlockParamUnion {
	return anthropic.BetaContentBlockParamUnion{OfImage: &anthropic.BetaImageBlockParam{
		Source: anthropic.BetaImageBlockParamSourceUnion{
			OfBase64: &anthropic.BetaBase64ImageSourceParam{
				Data: base64.StdEncoding.EncodeToString(
					png,
				),
				MediaType: anthropic.BetaBase64ImageSourceMediaTypeImagePNG,
			},
		},
	}}
}

func (r *run) call(
	ctx context.Context,
	messages []anthropic.BetaMessageParam,
) (*anthropic.BetaMessage, error) {
	params := anthropic.BetaMessageNewParams{
		Model:     anthropic.Model(r.opts.Model),
		MaxTokens: maxTokens,
		System:    []anthropic.BetaTextBlockParam{{Text: prompt + "\n" + furnitureGuide()}},
		Messages:  messages,
		Tools:     tools(),
		// Caches the tools, system prompt and conversation so far; each turn
		// then pays full price only for what it adds.
		CacheControl: anthropic.NewBetaCacheControlEphemeralParam(),
		OutputConfig: anthropic.BetaOutputConfigParam{
			Effort:     anthropic.BetaOutputConfigEffort(r.opts.Effort),
			TaskBudget: anthropic.BetaTokenTaskBudgetParam{Total: taskBudget},
		},
		// Crops pile up fast; drop all but the most recent tool results.
		// get_trace recovers a submission that has been dropped.
		ContextManagement: anthropic.BetaContextManagementConfigParam{
			Edits: []anthropic.BetaContextManagementConfigEditUnionParam{{
				OfClearToolUses20250919: &anthropic.BetaClearToolUses20250919EditParam{
					Keep: anthropic.BetaToolUsesKeepParam{Value: keepToolUses},
				},
			}},
		},
		Fallbacks: anthropic.BetaFallbacksParamOfDefault(),
		Betas: []anthropic.AnthropicBeta{
			anthropic.AnthropicBetaContextManagement2025_06_27,
			anthropic.AnthropicBetaTaskBudgets2026_03_13,
			anthropic.AnthropicBetaServerSideFallback2026_07_01,
		},
	}
	stream := r.opts.Client.Beta.Messages.NewStreaming(ctx, params)
	defer stream.Close() //nolint:errcheck // nothing to do with a close error
	msg := anthropic.BetaMessage{}
	for stream.Next() {
		if err := msg.Accumulate(stream.Current()); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrModel, err)
		}
	}
	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrModel, err)
	}
	return &msg, nil
}

func (r *run) account(u anthropic.BetaUsage) {
	r.result.Usage.Input += u.InputTokens
	r.result.Usage.Output += u.OutputTokens
	r.result.Usage.CacheRead += u.CacheReadInputTokens
	r.result.Usage.CacheWrite += u.CacheCreationInputTokens
	p, t := r.opts.Prices, r.result.Usage
	r.result.CostUSD = (float64(t.Input)*p.Input + float64(t.Output)*p.Output +
		float64(t.CacheRead)*p.CacheRead + float64(t.CacheWrite)*p.CacheWrite) / 1e6
}

// finish settles the result: the last valid submission is what gets
// published, whether the model called finish or a limit stopped it.
func (r *run) finish(err error) (*Result, error) {
	if err != nil {
		return &r.result, err
	}
	if r.lastGood == nil {
		if r.last != nil {
			r.result.Problems = r.last.problems
		}
		reason := r.result.Stopped
		if reason == "" {
			reason = "no valid submission"
		}
		return &r.result, fmt.Errorf("%w: %s", ErrNoTrace, reason)
	}
	g := r.lastGood
	r.result.Trace, r.result.Plan, r.result.PlanJSON = g.trace, g.plan, g.planJSON
	png, err := raster.EncodePNG(g.overlay)
	if err != nil {
		return &r.result, fmt.Errorf("encode overlay: %w", err)
	}
	r.result.Overlay = png
	if r.result.Stopped != "" {
		r.result.Caveats = append(r.result.Caveats, "Tracing stopped early: "+r.result.Stopped+".")
		if !r.finished {
			r.result.Summary = g.trace.Title
		}
	}
	return &r.result, nil
}

type transcriptLine struct {
	Turn    int      `json:"turn"`
	Stop    string   `json:"stop"`
	Text    []string `json:"text,omitempty"`
	Calls   []string `json:"calls,omitempty"`
	Results []string `json:"results,omitempty"`
	CostUSD float64  `json:"costUsd"`
}

// log writes a turn to the transcript: the model's text, the tools it called
// and the text of their results. Images and full trace inputs are left out;
// trace.json holds the final trace.
func (r *run) log(msg *anthropic.BetaMessage, reply []anthropic.BetaContentBlockParamUnion) {
	if r.opts.Transcript == nil {
		return
	}
	line := transcriptLine{
		Turn:    r.result.Turns,
		Stop:    string(msg.StopReason),
		CostUSD: math.Round(r.result.CostUSD*1e4) / 1e4,
	}
	for _, b := range msg.Content {
		switch v := b.AsAny().(type) {
		case anthropic.BetaTextBlock:
			line.Text = append(line.Text, v.Text)
		case anthropic.BetaToolUseBlock:
			in := v.JSON.Input.Raw()
			if v.Name == toolSubmit {
				in = fmt.Sprintf("(%d bytes)", len(in))
			}
			line.Calls = append(line.Calls, v.Name+" "+in)
		}
	}
	for _, b := range reply {
		if b.OfToolResult != nil {
			for _, c := range b.OfToolResult.Content {
				if c.OfText != nil {
					line.Results = append(line.Results, c.OfText.Text)
				}
			}
		}
		if b.OfText != nil {
			line.Results = append(line.Results, b.OfText.Text)
		}
	}
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(line)
	_, _ = r.opts.Transcript.Write(buf.Bytes())
}
