package trace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"

	"github.com/weaveplatform/spaceweave/internal/config"
	"github.com/weaveplatform/spaceweave/internal/fakeclaude"
	"github.com/weaveplatform/spaceweave/internal/plan"
	"github.com/weaveplatform/spaceweave/internal/raster"
)

// box is a 200×100 image: a 10 m × 5 m room at 20 px/m, walls 4 px thick.
func box(t *testing.T) *raster.Source {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 200, 100))
	for y := range 100 {
		for x := range 200 {
			c := color.RGBA{255, 255, 255, 255}
			if x < 4 || x >= 196 || y < 4 || y >= 96 {
				c = color.RGBA{0, 0, 0, 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	s, err := raster.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func validTrace() map[string]any {
	return map[string]any{
		"title":       "Box",
		"calibration": map[string]any{"pxPerMetre": []float64{20, 20}, "evidence": "200 px wall labelled 10 m"},
		"thickness":   map[string]any{"exterior": 0.2, "interior": 0.1, "glass": 0.05},
		"entrance":    []float64{2, 50},
		"walls": []any{
			map[string]any{"id": "n", "kind": "exterior", "from": []float64{2, 2}, "to": []float64{198, 2}},
			map[string]any{"id": "s", "kind": "exterior", "from": []float64{2, 98}, "to": []float64{198, 98}},
			map[string]any{"id": "w", "kind": "exterior", "from": []float64{2, 2}, "to": []float64{2, 98},
				"openings": []any{map[string]any{"a": 40, "b": 58, "kind": "door", "swing": "E"}}},
			map[string]any{"id": "e", "kind": "exterior", "from": []float64{198, 2}, "to": []float64{198, 98}},
		},
		"rooms": []any{map[string]any{"id": "r", "name": "Room", "zone": "work", "floor": "carpet",
			"polygon": [][]float64{{2, 2}, {198, 2}, {198, 98}, {2, 98}}}},
		"furniture": []any{map[string]any{"type": "desk", "x": 100, "y": 50, "w": 1.6, "d": 0.8}},
	}
}

func tool(name string, input any) fakeclaude.Block { return fakeclaude.Block{Name: name, Input: input} }

func calls(blocks ...fakeclaude.Block) fakeclaude.Reply {
	return fakeclaude.Reply{Content: blocks, StopReason: "tool_use", Usage: fakeclaude.Usage{Input: 1000, Output: 200}}
}

type fakeValidator struct {
	errs [][]string // one verdict per call; the last repeats
	err  error
	n    int
}

func (v *fakeValidator) Validate(_ context.Context, planJSON []byte) ([]string, error) {
	var p map[string]any
	if err := json.Unmarshal(planJSON, &p); err != nil {
		return nil, err
	}
	if v.err != nil {
		return nil, v.err
	}
	i := min(v.n, len(v.errs)-1)
	v.n++
	if i < 0 {
		return nil, nil
	}
	return v.errs[i], nil
}

func options(srv *fakeclaude.Server, v plan.Validator) Options {
	return Options{
		Client:    anthropic.NewClient(option.WithBaseURL(srv.URL), option.WithAPIKey("test"), option.WithMaxRetries(0)),
		Model:     "claude-opus-5-5",
		Effort:    "high",
		Limits:    config.Limits{MaxTurns: 20, MaxSubmits: 5, MaxUSD: 10, Timeout: time.Minute},
		Prices:    config.Prices{Input: 4, Output: 20, CacheRead: 0.2, CacheWrite: 5},
		Validator: v,
	}
}

func job(t *testing.T) Job {
	return Job{Image: box(t), Title: "Box", Meta: plan.Meta{URL: "generated/box/source.png", Credit: "test", SizePx: [2]int{200, 100}}}
}

// toolResults decodes the tool_result blocks of a request's last message.
type resultBlock struct {
	Type      string `json:"type"`
	ToolUseID string `json:"tool_use_id"`
	IsError   bool   `json:"is_error"`
	Text      string `json:"text"`
	Content   []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func lastUserBlocks(t *testing.T, req fakeclaude.Request) []resultBlock {
	t.Helper()
	var m struct {
		Role    string        `json:"role"`
		Content []resultBlock `json:"content"`
	}
	if err := json.Unmarshal(req.Messages[len(req.Messages)-1], &m); err != nil {
		t.Fatal(err)
	}
	if m.Role != "user" {
		t.Fatalf("last message is %s", m.Role)
	}
	return m.Content
}

func (b resultBlock) text() string {
	var parts []string
	for _, c := range b.Content {
		parts = append(parts, c.Text)
	}
	return b.Text + strings.Join(parts, "\n")
}

func (b resultBlock) hasImage() bool {
	return slices.ContainsFunc(b.Content, func(c struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}) bool {
		return c.Type == "image"
	})
}

func TestHappyPath(t *testing.T) {
	x0, y0, x1, y1 := 0, 0, 50, 50
	srv := fakeclaude.New(func(n int, _ fakeclaude.Request) fakeclaude.Reply {
		switch n {
		case 0:
			return calls(
				tool(toolView, map[string]any{}),
				tool(toolView, map[string]any{"x0": x0, "y0": y0, "x1": x1, "y1": y1, "grid": 10}),
				tool(toolScan, map[string]any{"axis": "row", "at": 50, "from": 0, "to": 199}),
				tool(toolScan, map[string]any{"axis": "col", "at": 100, "from": 0, "to": 99}),
				tool(toolWalls, map[string]any{"minThickness": 2, "maxThickness": 10, "minLength": 50}),
			)
		case 1:
			return calls(tool(toolSubmit, map[string]any{"trace": validTrace()}))
		case 2:
			return calls(tool(toolOverlay, map[string]any{"x0": 0, "y0": 0, "x1": 100, "y1": 100}), tool(toolGetTrace, map[string]any{}))
		default:
			return fakeclaude.Reply{
				Content:    []fakeclaude.Block{{Text: "Done."}, tool(toolFinish, map[string]any{"confidence": 1.4, "summary": " A box. ", "caveats": []string{"Furniture guessed"}})},
				StopReason: "tool_use", Usage: fakeclaude.Usage{Input: 10, Output: 10, CacheRead: 5000, CacheWrite: 1000},
			}
		}
	})
	defer srv.Close()
	var transcript bytes.Buffer
	opts := options(srv, &fakeValidator{})
	opts.Transcript = &transcript

	res, err := Run(context.Background(), opts, job(t))
	if err != nil {
		t.Fatal(err)
	}
	if res.Confidence != 1 || res.Summary != "A box." || len(res.Caveats) != 1 || res.Stopped != "" || res.Turns != 4 || res.Submits != 1 {
		t.Fatalf("result %+v", res)
	}
	if res.Plan == nil || res.Plan.Width != 9.8 || !bytes.HasPrefix(res.Overlay, []byte("\x89PNG")) || !json.Valid(res.PlanJSON) {
		t.Fatalf("plan %+v", res.Plan)
	}
	want := Usage{Input: 3010, Output: 610, CacheRead: 5000, CacheWrite: 1000}
	if res.Usage != want {
		t.Fatalf("usage %+v", res.Usage)
	}
	if cost := (3010*4 + 610*20 + 5000*0.2 + 1000*5) / 1e6; res.CostUSD != cost {
		t.Fatalf("cost %v, want %v", res.CostUSD, cost)
	}

	reqs := srv.Requests()
	first := reqs[0]
	if first.Model != "claude-opus-5-5" || !first.Stream {
		t.Fatalf("first request: model %s stream %v", first.Model, first.Stream)
	}
	for _, beta := range []string{"context-management-2025-06-27", "task-budgets-2026-03-13", "server-side-fallback-2026-07-01"} {
		if !strings.Contains(first.Header.Get("anthropic-beta"), beta) {
			t.Errorf("missing beta %s in %q", beta, first.Header.Get("anthropic-beta"))
		}
	}
	var body struct {
		System       []struct{ Text string } `json:"system"`
		Tools        []map[string]any        `json:"tools"`
		Fallbacks    any                     `json:"fallbacks"`
		CacheControl map[string]any          `json:"cache_control"`
		OutputConfig struct {
			Effort     string         `json:"effort"`
			TaskBudget map[string]any `json:"task_budget"`
		} `json:"output_config"`
		ContextManagement struct {
			Edits []map[string]any `json:"edits"`
		} `json:"context_management"`
	}
	if err := json.Unmarshal(first.Raw, &body); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body.System[0].Text, "# Furniture types") || !strings.Contains(body.System[0].Text, "`roundTable`: needs r; defaults h=0.74") {
		t.Fatal("system prompt lacks the furniture guide")
	}
	if len(body.Tools) != 7 || body.Fallbacks != "default" || body.CacheControl["type"] != "ephemeral" ||
		body.OutputConfig.Effort != "high" || body.OutputConfig.TaskBudget["total"] != float64(taskBudget) ||
		body.ContextManagement.Edits[0]["type"] != "clear_tool_uses_20250919" {
		t.Fatalf("request params: %s", first.Raw[:min(len(first.Raw), 400)])
	}
	submitTool := body.Tools[slices.IndexFunc(body.Tools, func(m map[string]any) bool { return m["name"] == toolSubmit })]
	if submitTool["eager_input_streaming"] != true {
		t.Fatal("submit_trace should stream its input")
	}

	views := lastUserBlocks(t, reqs[1])
	if len(views) != 5 || !views[0].hasImage() || !strings.Contains(views[1].text(), "x 0–50, y 0–50 at 4.00×") {
		t.Fatalf("view results: %+v", views)
	}
	if !strings.Contains(views[2].text(), "0–3 (4)") || !strings.Contains(views[2].text(), "196–199 (4)") {
		t.Fatalf("scan: %s", views[2].text())
	}
	if !strings.Contains(views[4].text(), "h (0,2)→(199,2) t=4") {
		t.Fatalf("walls: %s", views[4].text())
	}
	sub := lastUserBlocks(t, reqs[2])[0]
	if sub.IsError || !strings.Contains(sub.text(), "No problems") || !strings.Contains(sub.text(), "9.80 × 4.80 m; 4 walls, 1 openings, 1 rooms, 1 furniture items") || !sub.hasImage() {
		t.Fatalf("submit: %+v", sub)
	}
	after := lastUserBlocks(t, reqs[3])
	if !after[0].hasImage() || !strings.Contains(after[1].text(), `"title":"Box"`) {
		t.Fatalf("overlay and get_trace: %+v", after)
	}

	lines := strings.Split(strings.TrimSpace(transcript.String()), "\n")
	if len(lines) != 4 || !strings.Contains(lines[1], "submit_trace (") || !strings.Contains(lines[3], `"text":["Done."]`) {
		t.Fatalf("transcript:\n%s", transcript.String())
	}
}

func TestProblemsAreReportedAndFinishIsHeldBack(t *testing.T) {
	bad := validTrace()
	bad["rooms"] = []any{map[string]any{"id": "r", "name": "", "zone": "lounge", "floor": "carpet", "polygon": [][]float64{{0, 0}, {1, 0}, {1, 1}}}}
	srv := fakeclaude.New(func(n int, _ fakeclaude.Request) fakeclaude.Reply {
		switch n {
		case 0:
			return calls(tool(toolFinish, map[string]any{"confidence": 0.5, "summary": "x", "caveats": []string{}}),
				tool(toolOverlay, map[string]any{}), tool(toolGetTrace, map[string]any{}))
		case 1:
			return calls(tool(toolSubmit, map[string]any{"trace": bad}))
		case 2:
			return calls(tool(toolSubmit, map[string]any{"trace": validTrace()}))
		case 3:
			return calls(tool(toolFinish, map[string]any{"confidence": 0.5, "summary": "x", "caveats": []string{}}))
		default:
			return fakeclaude.Reply{Content: []fakeclaude.Block{{Text: "I'm done."}}}
		}
	})
	defer srv.Close()
	v := &fakeValidator{errs: [][]string{{"room r has a corner outside the outline"}}}
	res, err := Run(context.Background(), options(srv, v), job(t))
	if !errors.Is(err, ErrNoTrace) || res.Stopped != StopStalled {
		t.Fatalf("err %v, stopped %q", err, res.Stopped)
	}
	if len(res.Problems) != 1 || !strings.Contains(res.Problems[0], "outside the outline") {
		t.Fatalf("problems of the last submission are kept: %v", res.Problems)
	}
	reqs := srv.Requests()
	early := lastUserBlocks(t, reqs[1])
	for _, b := range early {
		if !b.IsError || !strings.Contains(b.text(), "nothing submitted yet") && !strings.Contains(b.text(), "isn't valid") {
			t.Fatalf("calls before any submission are errors: %+v", b)
		}
	}
	structural := lastUserBlocks(t, reqs[2])[0]
	if structural.IsError || !strings.Contains(structural.text(), "room r has no name") || !strings.Contains(structural.text(), "zone must be one of") || !structural.hasImage() {
		t.Fatalf("structural problems come back with the overlay: %s", structural.text())
	}
	validated := lastUserBlocks(t, reqs[3])[0]
	if !strings.Contains(validated.text(), "1 problems") {
		t.Fatalf("validator problems: %s", validated.text())
	}
	if refused := lastUserBlocks(t, reqs[4])[0]; !refused.IsError {
		t.Fatal("finish after an invalid submission is refused")
	}
	// Three replies with no tool calls: two nudges, then stop.
	if len(reqs) != 7 || !strings.Contains(lastUserBlocks(t, reqs[6])[0].Text, "Keep working") {
		t.Fatalf("%d requests", len(reqs))
	}
}

func TestStopsWithAValidTraceStillPublish(t *testing.T) {
	cases := []struct {
		name   string
		limits func(*config.Limits)
		then   fakeclaude.Reply
		want   string
	}{
		{"spend", func(l *config.Limits) { l.MaxUSD = 0.001 }, calls(tool(toolGetTrace, map[string]any{})), StopSpend},
		{"turns", func(l *config.Limits) { l.MaxTurns = 2 }, calls(tool(toolGetTrace, map[string]any{})), StopTurns},
		{"context", func(*config.Limits) {}, fakeclaude.Reply{StopReason: "model_context_window_exceeded"}, StopContext},
		{"refusal", func(*config.Limits) {}, fakeclaude.Reply{StopReason: "refusal", Category: "cyber"}, StopRefusal + " (cyber)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := fakeclaude.New(func(n int, _ fakeclaude.Request) fakeclaude.Reply {
				if n == 0 {
					return calls(tool(toolSubmit, map[string]any{"trace": validTrace()}))
				}
				return c.then
			})
			defer srv.Close()
			opts := options(srv, &fakeValidator{})
			c.limits(&opts.Limits)
			res, err := Run(context.Background(), opts, job(t))
			if err != nil {
				t.Fatal(err)
			}
			if res.Stopped != c.want || res.Plan == nil || res.Summary != "Box" || !strings.Contains(strings.Join(res.Caveats, " "), "stopped early: "+c.want) {
				t.Fatalf("%+v", res)
			}
		})
	}
}

func TestSubmissionLimit(t *testing.T) {
	bad := validTrace()
	bad["title"] = ""
	srv := fakeclaude.New(func(n int, _ fakeclaude.Request) fakeclaude.Reply {
		return calls(tool(toolSubmit, map[string]any{"trace": bad}))
	})
	defer srv.Close()
	opts := options(srv, &fakeValidator{})
	opts.Limits.MaxSubmits = 2
	res, err := Run(context.Background(), opts, job(t))
	if !errors.Is(err, ErrNoTrace) || res.Stopped != StopSubmits || res.Submits != 2 {
		t.Fatalf("%v %+v", err, res)
	}

	// With a valid last submission the model is told to finish, and a further
	// submission is refused.
	srv2 := fakeclaude.New(func(n int, _ fakeclaude.Request) fakeclaude.Reply {
		if n == 0 {
			return calls(tool(toolSubmit, map[string]any{"trace": validTrace()}))
		}
		if n == 1 {
			return calls(tool(toolSubmit, map[string]any{"trace": validTrace()}))
		}
		return calls(tool(toolFinish, map[string]any{"confidence": 0.9, "summary": "Box", "caveats": []string{}}))
	})
	defer srv2.Close()
	opts = options(srv2, &fakeValidator{})
	opts.Limits.MaxSubmits = 1
	res, err = Run(context.Background(), opts, job(t))
	if err != nil || res.Confidence != 0.9 {
		t.Fatalf("%v %+v", err, res)
	}
	reqs := srv2.Requests()
	told := lastUserBlocks(t, reqs[1])
	if !strings.Contains(told[len(told)-1].Text, "last submission") {
		t.Fatalf("not told to finish: %+v", told)
	}
	if refused := lastUserBlocks(t, reqs[2])[0]; !refused.IsError || !strings.Contains(refused.text(), "no submissions left") {
		t.Fatalf("%+v", refused)
	}
}

func TestBadToolCalls(t *testing.T) {
	srv := fakeclaude.New(func(n int, _ fakeclaude.Request) fakeclaude.Reply {
		switch n {
		case 0:
			return calls(
				fakeclaude.Block{Name: toolSubmit, RawInput: `{"trace": {"title": "cut of`},
				tool(toolSubmit, map[string]any{"trace": map[string]any{"title": "Walls only"}}),
				tool(toolView, map[string]any{"x0": 1}),
				tool(toolView, map[string]any{"x0": 500, "y0": 500, "x1": 600, "y1": 600}),
				tool(toolView, map[string]any{"zoom": 3}),
				tool(toolScan, map[string]any{"axis": "diagonal", "at": 1, "from": 0, "to": 9}),
				tool(toolWalls, map[string]any{"minThickness": 5, "maxThickness": 2, "minLength": 10}),
				tool(toolWalls, map[string]any{"minThickness": 1, "maxThickness": 2, "minLength": 10, "grid": 5}),
				tool(toolWalls, map[string]any{"x0": 900, "y0": 900, "x1": 950, "y1": 950, "minThickness": 1, "maxThickness": 2, "minLength": 10}),
				tool(toolWalls, map[string]any{"x0": 1, "minThickness": 1, "maxThickness": 2, "minLength": 10}),
				tool("teleport", map[string]any{}),
				tool(toolFinish, map[string]any{"confidence": "high", "summary": "x", "caveats": []string{}}),
				tool(toolScan, map[string]any{"axis": "row", "at": 1, "from": 0, "to": "end"}),
			)
		case 1:
			// Cut off mid tool call.
			return fakeclaude.Reply{Content: []fakeclaude.Block{tool(toolSubmit, map[string]any{"trace": map[string]any{}})}, StopReason: "max_tokens"}
		default:
			return fakeclaude.Reply{Status: http.StatusBadRequest}
		}
	})
	defer srv.Close()
	res, err := Run(context.Background(), options(srv, &fakeValidator{}), job(t))
	if !errors.Is(err, ErrModel) || res.Turns != 3 {
		t.Fatalf("a failed request ends the run: %v %+v", err, res)
	}
	reqs := srv.Requests()
	results := lastUserBlocks(t, reqs[1])
	if len(results) != 13 {
		t.Fatalf("every call gets a result: %d", len(results))
	}
	wants := []string{
		"input lacks trace", "input lacks calibration, thickness, entrance, walls, rooms, furniture", "give all of x0, y0, x1, y1", "region does not overlap the image (the image is 200 × 100)",
		`unknown field "zoom"`, "axis must be row or col", "need 1 ≤ minThickness", "takes no grid", "region does not overlap",
		"give all of x0", `unknown tool "teleport"`, "not valid for this tool", "not valid for this tool",
	}
	for i, w := range wants {
		if !results[i].IsError || !strings.Contains(results[i].text(), w) {
			t.Errorf("result %d: want error containing %q, got %+v", i, w, results[i])
		}
	}
	cut := lastUserBlocks(t, reqs[2])
	if !cut[0].IsError || !strings.Contains(cut[0].text(), "cut off") || !strings.Contains(cut[1].Text, "hit the output limit") {
		t.Fatalf("max_tokens: %+v", cut)
	}
	if res.Submits != 0 {
		t.Fatal("rejected inputs don't use up submissions")
	}
}

func TestValidatorFailureEndsTheRun(t *testing.T) {
	srv := fakeclaude.New(func(int, fakeclaude.Request) fakeclaude.Reply {
		return calls(tool(toolSubmit, map[string]any{"trace": validTrace()}))
	})
	defer srv.Close()
	boom := errors.New("node: not found")
	res, err := Run(context.Background(), options(srv, &fakeValidator{err: boom}), job(t))
	if !errors.Is(err, boom) || res.Turns != 1 {
		t.Fatalf("%v %+v", err, res)
	}
}

func TestTimeout(t *testing.T) {
	srv := fakeclaude.New(func(int, fakeclaude.Request) fakeclaude.Reply {
		time.Sleep(200 * time.Millisecond)
		return calls(tool(toolGetTrace, map[string]any{}))
	})
	defer srv.Close()
	opts := options(srv, &fakeValidator{})
	opts.Limits.Timeout = 50 * time.Millisecond
	res, err := Run(context.Background(), opts, job(t))
	if !errors.Is(err, ErrNoTrace) || res.Stopped != StopTimeout {
		t.Fatalf("%v %+v", err, res)
	}
}

func TestFeedbackAndPreviousTraceOpenTheRun(t *testing.T) {
	srv := fakeclaude.New(func(int, fakeclaude.Request) fakeclaude.Reply {
		return fakeclaude.Reply{StopReason: "refusal"}
	})
	defer srv.Close()
	j := job(t)
	j.Feedback = "The kitchen is missing."
	j.Previous = &plan.Trace{Title: "Earlier"}
	_, err := Run(context.Background(), options(srv, &fakeValidator{}), j)
	if !errors.Is(err, ErrNoTrace) || !strings.Contains(err.Error(), StopRefusal) {
		t.Fatal(err)
	}
	opening := string(srv.Requests()[0].Messages[0])
	for _, want := range []string{"The kitchen is missing.", `\"title\":\"Earlier\"`, "200 × 100 pixels", `suggests the title \"Box\"`, `"type":"image"`} {
		if !strings.Contains(opening, want) {
			t.Errorf("opening message lacks %s", want)
		}
	}
}

func TestToolsDescribeTheirInputs(t *testing.T) {
	ts := tools()
	names := make([]string, 0, len(ts))
	for _, u := range ts {
		names = append(names, u.OfTool.Name)
		schema, err := json.Marshal(u.OfTool.InputSchema)
		if err != nil || !strings.Contains(string(schema), `"additionalProperties":false`) {
			t.Fatalf("%s schema: %s %v", u.OfTool.Name, schema, err)
		}
	}
	if !slices.Equal(names, []string{toolView, toolScan, toolWalls, toolSubmit, toolOverlay, toolGetTrace, toolFinish}) {
		t.Fatal(names)
	}
	// Every furniture type Check accepts is offered by the schema.
	schema, _ := json.Marshal(traceSchema())
	for name := range plan.Types {
		if !strings.Contains(string(schema), `"`+name+`"`) {
			t.Errorf("schema lacks furniture type %s", name)
		}
	}
}
