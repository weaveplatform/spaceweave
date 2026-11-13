package trace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"math"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"

	"github.com/weaveplatform/spaceweave/internal/plan"
	"github.com/weaveplatform/spaceweave/internal/raster"
)

// errTool is a mistake in a tool call, reported back to the model as an
// error result for it to correct.
var errTool = errors.New("tool call rejected")

// validateTimeout bounds one run of the node validator.
const validateTimeout = time.Minute

// maxWalls bounds detect_walls' reply.
const maxWalls = 150

type content = []anthropic.BetaToolResultBlockParamContentUnion

func text(s string) anthropic.BetaToolResultBlockParamContentUnion {
	return anthropic.BetaToolResultBlockParamContentUnion{
		OfText: &anthropic.BetaTextBlockParam{Text: s},
	}
}

func pngContent(png []byte) anthropic.BetaToolResultBlockParamContentUnion {
	return anthropic.BetaToolResultBlockParamContentUnion{OfImage: imageBlock(png).OfImage}
}

// tools runs every tool call in msg and returns their results.
func (r *run) tools(
	ctx context.Context,
	msg *anthropic.BetaMessage,
) []anthropic.BetaContentBlockParamUnion {
	var out []anthropic.BetaContentBlockParamUnion
	for _, b := range msg.Content {
		use, ok := b.AsAny().(anthropic.BetaToolUseBlock)
		if !ok {
			continue
		}
		res := anthropic.BetaToolResultBlockParam{ToolUseID: use.ID}
		var c content
		var err error
		if msg.StopReason == anthropic.BetaStopReasonMaxTokens {
			err = fmt.Errorf("%w: the input was cut off by the output limit", errTool)
		} else {
			c, err = r.dispatch(ctx, use.Name, []byte(use.JSON.Input.Raw()))
		}
		if err != nil {
			res.IsError = anthropic.Bool(true)
			c = content{text(err.Error())}
		}
		res.Content = c
		out = append(out, anthropic.BetaContentBlockParamUnion{OfToolResult: &res})
	}
	return out
}

// decode reads a tool input strictly: with eager input streaming the API
// doesn't validate inputs, so a truncated or misspelt field must be caught
// here rather than read as a zero value.
func decode(raw []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%w: input is not valid for this tool: %w", errTool, err)
	}
	return nil
}

// required lists each tool's required input fields, and the trace's. The SDK
// turns a tool input it can't parse (one cut off mid-string, say) into {},
// which decodes cleanly into zero values, so a missing field is the only sign.
var required = func() map[string][]string {
	m := map[string][]string{}
	for _, t := range tools() {
		m[t.OfTool.Name] = t.OfTool.InputSchema.Required
	}
	m["trace"] = traceSchema()["required"].([]string) //nolint:forcetypeassert // built in this package
	return m
}()

func missing(raw []byte, keys []string) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return fmt.Errorf("%w: input is not valid for this tool: %w", errTool, err)
	}
	var gone []string
	for _, k := range keys {
		if _, ok := fields[k]; !ok {
			gone = append(gone, k)
		}
	}
	if len(gone) > 0 {
		return fmt.Errorf("%w: input lacks %s (a long input may have been cut off; send it again)",
			errTool, strings.Join(gone, ", "))
	}
	if t, ok := fields["trace"]; ok && keys != nil {
		return missing(t, required["trace"])
	}
	return nil
}

func (r *run) dispatch(ctx context.Context, name string, raw []byte) (content, error) {
	if err := missing(raw, required[name]); err != nil {
		return nil, err
	}
	switch name {
	case toolView:
		return r.view(raw, r.job.Image.Image)
	case toolOverlay:
		if r.last == nil {
			return nil, fmt.Errorf("%w: nothing submitted yet", errTool)
		}
		return r.view(raw, r.last.overlay)
	case toolScan:
		return r.scan(raw)
	case toolWalls:
		return r.walls(raw)
	case toolSubmit:
		return r.submit(ctx, raw)
	case toolGetTrace:
		if r.last == nil {
			return nil, fmt.Errorf("%w: nothing submitted yet", errTool)
		}
		b, err := json.Marshal(r.last.trace)
		if err != nil {
			return nil, fmt.Errorf("encode trace: %w", err)
		}
		return content{text(string(b))}, nil
	case toolFinish:
		return r.finishTool(raw)
	default:
		return nil, fmt.Errorf("%w: unknown tool %q", errTool, name)
	}
}

type regionInput struct {
	X0   *int `json:"x0"`
	Y0   *int `json:"y0"`
	X1   *int `json:"x1"`
	Y1   *int `json:"y1"`
	Grid int  `json:"grid"`
}

// rect is the requested region, or the whole image when none is given.
func (in regionInput) rect(b image.Rectangle) (image.Rectangle, error) {
	if in.X0 == nil && in.Y0 == nil && in.X1 == nil && in.Y1 == nil {
		return b, nil
	}
	if in.X0 == nil || in.Y0 == nil || in.X1 == nil || in.Y1 == nil {
		return image.Rectangle{}, fmt.Errorf(
			"%w: give all of x0, y0, x1, y1, or none for the whole image",
			errTool,
		)
	}
	return image.Rect(*in.X0, *in.Y0, *in.X1, *in.Y1), nil
}

func (r *run) view(raw []byte, img image.Image) (content, error) {
	var in regionInput
	if err := decode(raw, &in); err != nil {
		return nil, err
	}
	rect, err := in.rect(img.Bounds())
	if err != nil {
		return nil, err
	}
	v, err := raster.Crop(img, rect, in.Grid)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: %w (the image is %d × %d)",
			errTool,
			err,
			img.Bounds().Dx(),
			img.Bounds().Dy(),
		)
	}
	png, err := raster.EncodePNG(v.Image)
	if err != nil {
		return nil, fmt.Errorf("encode view: %w", err)
	}
	desc := fmt.Sprintf("x %d–%d, y %d–%d at %.2f×; rulers are source pixels.",
		v.Region.Min.X, v.Region.Max.X, v.Region.Min.Y, v.Region.Max.Y, v.Scale)
	return content{text(desc), pngContent(png)}, nil
}

func (r *run) scan(raw []byte) (content, error) {
	var in struct {
		Axis string `json:"axis"`
		At   int    `json:"at"`
		From int    `json:"from"`
		To   int    `json:"to"`
	}
	if err := decode(raw, &in); err != nil {
		return nil, err
	}
	var runs []raster.Run
	switch in.Axis {
	case "row":
		runs = r.job.Image.ScanRow(in.At, in.From, in.To)
	case "col":
		runs = r.job.Image.ScanCol(in.At, in.From, in.To)
	default:
		return nil, fmt.Errorf("%w: axis must be row or col", errTool)
	}
	var b strings.Builder
	fmt.Fprintf(
		&b,
		"%d ink runs along %s %d from %d to %d (start–end inclusive, length):\n",
		len(runs),
		in.Axis,
		in.At,
		in.From,
		in.To,
	)
	for _, run := range runs {
		fmt.Fprintf(&b, "%d–%d (%d)\n", run.Start, run.End, run.Len())
	}
	if len(runs) == raster.MaxRuns {
		b.WriteString("Stopped at the run limit; scan a shorter span.\n")
	}
	return content{text(b.String())}, nil
}

func (r *run) walls(raw []byte) (content, error) {
	var in struct {
		regionInput
		MinThickness int `json:"minThickness"`
		MaxThickness int `json:"maxThickness"`
		MinLength    int `json:"minLength"`
	}
	if err := decode(raw, &in); err != nil {
		return nil, err
	}
	if in.Grid != 0 {
		return nil, fmt.Errorf("%w: detect_walls takes no grid", errTool)
	}
	rect, err := in.rect(r.job.Image.Image.Bounds())
	if err != nil {
		return nil, err
	}
	if in.MinThickness < 1 || in.MaxThickness < in.MinThickness || in.MinLength < 1 {
		return nil, fmt.Errorf(
			"%w: need 1 ≤ minThickness ≤ maxThickness and minLength ≥ 1",
			errTool,
		)
	}
	ws, err := r.job.Image.DetectWalls(rect, raster.WallLimits{
		MinThickness: in.MinThickness,
		MaxThickness: in.MaxThickness,
		MinLength:    in.MinLength,
		Max:          maxWalls,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errTool, err)
	}
	var b strings.Builder
	fmt.Fprintf(
		&b,
		"%d candidates (axis, centreline from → to, thickness px), longest first:\n",
		len(ws),
	)
	for _, w := range ws {
		fmt.Fprintf(
			&b,
			"%s (%d,%d)→(%d,%d) t=%d\n",
			w.Axis,
			w.From[0],
			w.From[1],
			w.To[0],
			w.To[1],
			w.Thickness,
		)
	}
	if len(ws) == maxWalls {
		b.WriteString("Stopped at the candidate limit; narrow the region or raise minLength.\n")
	}
	return content{text(b.String())}, nil
}

func (r *run) submit(ctx context.Context, raw []byte) (content, error) {
	if r.result.Submits >= r.opts.Limits.MaxSubmits {
		return nil, fmt.Errorf(
			"%w: no submissions left; call finish if your last one was valid",
			errTool,
		)
	}
	var in struct {
		Trace plan.Trace `json:"trace"`
	}
	if err := decode(raw, &in); err != nil {
		return nil, err
	}
	r.result.Submits++
	s := &submission{trace: &in.Trace}
	s.problems = plan.Check(s.trace)
	s.overlay = raster.Overlay(r.job.Image.Image, plan.Shapes(s.trace))
	r.last = s

	var b strings.Builder
	fmt.Fprintf(&b, "Submission %d of %d. ", r.result.Submits, r.opts.Limits.MaxSubmits)
	if len(s.problems) == 0 {
		p, err := plan.Convert(s.trace, r.job.Meta)
		if err != nil {
			s.problems = []string{err.Error()}
		} else {
			s.plan = p
			s.planJSON, err = json.Marshal(p)
			if err != nil {
				return nil, fmt.Errorf("encode plan: %w", err)
			}
			ctx, cancel := context.WithTimeout(ctx, validateTimeout)
			defer cancel()
			verdict, err := r.opts.Validator.Validate(ctx, s.planJSON)
			if err != nil {
				// The model can't fix a broken validator; stop rather than let
				// it spend the budget resubmitting.
				r.fatal = fmt.Errorf("validate plan: %w", err)
				return nil, r.fatal
			}
			s.problems = verdict
			fmt.Fprintf(&b, "In metres: %s. ", plan.Describe(p))
		}
	}
	if s.valid() {
		r.lastGood = s
		b.WriteString(
			"No problems. Check the overlay below; fix anything misaligned and resubmit, or call finish.",
		)
	} else {
		fmt.Fprintf(
			&b,
			"%d problems (positions in the validator's errors are metres from the walls' top-left):\n- %s",
			len(s.problems),
			strings.Join(s.problems, "\n- "),
		)
	}
	fitted, _ := raster.Fit(s.overlay, raster.MaxEdge)
	png, err := raster.EncodePNG(fitted)
	if err != nil {
		return nil, fmt.Errorf("encode overlay: %w", err)
	}
	return content{text(b.String()), pngContent(png)}, nil
}

func (r *run) finishTool(raw []byte) (content, error) {
	var in struct {
		Confidence float64  `json:"confidence"`
		Summary    string   `json:"summary"`
		Caveats    []string `json:"caveats"`
	}
	if err := decode(raw, &in); err != nil {
		return nil, err
	}
	if !r.last.valid() {
		return nil, fmt.Errorf(
			"%w: your latest submission isn't valid; fix and resubmit it first",
			errTool,
		)
	}
	r.finished = true
	r.result.Confidence = math.Max(0, math.Min(1, in.Confidence))
	r.result.Summary = strings.TrimSpace(in.Summary)
	r.result.Caveats = in.Caveats
	return content{text("Finished.")}, nil
}
