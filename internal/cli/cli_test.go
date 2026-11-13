package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2/option"

	"github.com/weaveplatform/spaceweave/internal/catalog"
	"github.com/weaveplatform/spaceweave/internal/fakeclaude"
)

type okValidator struct{}

func (okValidator) Validate(context.Context, []byte) ([]string, error) { return nil, nil }

// boxPNG is a 200×100 room outline.
func boxPNG(t *testing.T, path string, shade uint8) {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, 200, 100))
	for y := range 100 {
		for x := range 200 {
			c := color.Gray{255}
			if x < 4 || x >= 196 || y < 4 || y >= 96 {
				c = color.Gray{shade}
			}
			img.SetGray(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func boxTrace() map[string]any {
	wall := func(id string, x0, y0, x1, y1 float64) map[string]any {
		return map[string]any{"id": id, "kind": "exterior", "from": []float64{x0, y0}, "to": []float64{x1, y1}}
	}
	return map[string]any{
		"title":       "Box",
		"calibration": map[string]any{"pxPerMetre": []float64{20, 20}, "evidence": "10 m wall"},
		"thickness":   map[string]any{"exterior": 0.2, "interior": 0.1, "glass": 0.05},
		"entrance":    []float64{2, 50},
		"walls": []any{
			wall("n", 2, 2, 198, 2), wall("s", 2, 98, 198, 98), wall("w", 2, 2, 2, 98), wall("e", 198, 2, 198, 98),
		},
		"rooms": []any{map[string]any{
			"id": "r", "name": "Room", "zone": "work", "floor": "carpet",
			"polygon": [][]float64{{2, 2}, {198, 2}, {198, 98}, {2, 98}},
		}},
		"furniture": []any{},
	}
}

// tracer scripts a model that submits the box and finishes.
func tracer(n int, _ fakeclaude.Request) fakeclaude.Reply {
	use := func(name string, in any) fakeclaude.Reply {
		return fakeclaude.Reply{
			Content: []fakeclaude.Block{{Name: name, Input: in}}, StopReason: "tool_use",
			Usage: fakeclaude.Usage{Input: 100000, Output: 10000},
		}
	}
	if n%2 == 0 {
		return use("submit_trace", map[string]any{"trace": boxTrace()})
	}
	return use("finish", map[string]any{"confidence": 0.75, "summary": "One room", "caveats": []string{}})
}

type env struct {
	t      *testing.T
	dir    string
	srv    *fakeclaude.Server
	vars   map[string]string
	stdout *bytes.Buffer
	stderr *syncBuffer
}

type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func newEnv(t *testing.T, script fakeclaude.Script) *env {
	t.Helper()
	dir := t.TempDir()
	srv := fakeclaude.New(script)
	t.Cleanup(srv.Close)
	return &env{
		t: t, dir: dir, srv: srv,
		vars: map[string]string{
			"SPACEWEAVE_INBOX":  filepath.Join(dir, "inbox"),
			"SPACEWEAVE_OUTPUT": filepath.Join(dir, "generated"),
		},
		stdout: &bytes.Buffer{}, stderr: &syncBuffer{},
	}
}

func (e *env) app() App {
	return App{
		Stdout: e.stdout, Stderr: e.stderr,
		Getenv: func(k string) (string, bool) { v, ok := e.vars[k]; return v, ok },
		ClientOptions: []option.RequestOption{
			option.WithBaseURL(e.srv.URL), option.WithAPIKey("test"), option.WithMaxRetries(0),
		},
		Validator: okValidator{},
	}
}

func (e *env) run(args ...string) int {
	e.t.Helper()
	e.stdout.Reset()
	return Execute(context.Background(), e.app(), args)
}

func (e *env) plans() []catalog.Entry {
	e.t.Helper()
	if code := e.run("plans", "list", "--json"); code != 0 {
		e.t.Fatalf("list: %d %s", code, e.stderr)
	}
	var out []catalog.Entry
	if err := json.Unmarshal(e.stdout.Bytes(), &out); err != nil {
		e.t.Fatal(err, e.stdout)
	}
	return out
}

func TestTraceAndReview(t *testing.T) {
	e := newEnv(t, tracer)
	img := filepath.Join(e.dir, "Box Plan.png")
	boxPNG(t, img, 0)

	if code := e.run("trace", img); code != 0 {
		t.Fatalf("trace: %d %s", code, e.stderr)
	}
	if !strings.Contains(e.stdout.String(), "draft  confidence 0.75") {
		t.Fatal(e.stdout)
	}
	ps := e.plans()
	id := ps[0].ID
	if len(ps) != 1 || ps[0].Status != catalog.Draft || ps[0].Attempts != 1 || ps[0].CostUSD != 1.2 {
		t.Fatalf("%+v", ps)
	}
	for _, f := range []string{"plan.json", "trace.json", "overlay.png", "report.json", "transcript.jsonl", "source.png"} {
		if _, err := os.Stat(filepath.Join(e.dir, "generated", id, f)); err != nil {
			t.Error(err)
		}
	}

	if code := e.run("trace", img); code != 0 || !strings.Contains(e.stdout.String(), "is already plan "+id+" (draft)") {
		t.Fatalf("%d %s", code, e.stdout)
	}
	if code := e.run("plans", "list"); code != 0 || !strings.Contains(e.stdout.String(), id+"  draft   0.75") {
		t.Fatalf("%d %q", code, e.stdout)
	}
	if code := e.run("plans", "retry", id); code != 1 || !strings.Contains(e.stderr.String(), "is draft") {
		t.Fatalf("retrying a draft: %d %s", code, e.stderr)
	}
	if code := e.run("plans", "approve", id); code != 0 || !strings.Contains(e.stdout.String(), "draft → approved") {
		t.Fatalf("%d %s", code, e.stdout)
	}
	if code := e.run("plans", "reject", id, "--note", "Missing the kitchen"); code != 0 {
		t.Fatalf("%d %s", code, e.stderr)
	}
	if p := e.plans()[0]; p.Status != catalog.Queued || p.Note != "Missing the kitchen" {
		t.Fatalf("%+v", p)
	}
	if code := e.run("plans", "approve", "nope"); code != 1 || !strings.Contains(e.stderr.String(), "no such plan") {
		t.Fatalf("%d %s", code, e.stderr)
	}
}

func TestFailedTraceAndRetry(t *testing.T) {
	e := newEnv(t, func(int, fakeclaude.Request) fakeclaude.Reply {
		return fakeclaude.Reply{StopReason: "refusal", Category: "cyber"}
	})
	img := filepath.Join(e.dir, "plan.webp")
	if err := os.WriteFile(img, []byte("RIFF not really webp"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code := e.run("trace", img); code != 1 || !strings.Contains(e.stderr.String(), "trace failed: read image") {
		t.Fatalf("%d %s", code, e.stderr)
	}
	id := e.plans()[0].ID
	if code := e.run("plans", "retry", id); code != 0 || !strings.Contains(e.stdout.String(), "failed → queued") {
		t.Fatalf("%d %s", code, e.stderr)
	}
	if p := e.plans()[0]; p.Status != catalog.Queued || p.Error != "" {
		t.Fatalf("%+v", p)
	}
	if code := e.run("plans", "reject", id); code != 1 {
		t.Fatal("rejecting a queued plan")
	}
}

func TestWatch(t *testing.T) {
	e := newEnv(t, tracer)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := filepath.Join(e.dir, "spaceweave.yaml")
	if err := os.WriteFile(cfg, []byte("settle: 20ms\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	inbox := filepath.Join(e.dir, "drop")
	app := e.app()
	app.Stdout = &syncBuffer{}
	exited := make(chan int, 1)
	go func() {
		exited <- Execute(ctx, app, []string{"watch", "--config", cfg, "--inbox", inbox, "--workers", "2", "--model", "claude-opus-5-5", "--max-usd", "3", "-v"})
	}()
	deadline := time.Now().Add(10 * time.Second)
	for _, err := os.Stat(inbox); err != nil; _, err = os.Stat(inbox) {
		if time.Now().After(deadline) {
			t.Fatal("inbox not created")
		}
		time.Sleep(10 * time.Millisecond)
	}
	boxPNG(t, filepath.Join(inbox, "first.png"), 0)
	boxPNG(t, filepath.Join(inbox, "copy.png"), 0) // same image: one plan
	catalogFile := filepath.Join(e.dir, "generated", "catalog.json")
	for {
		if time.Now().After(deadline) {
			t.Fatalf("no draft; log:\n%s", e.stderr)
		}
		var c catalog.Catalog
		if b, err := os.ReadFile(catalogFile); err == nil && json.Unmarshal(b, &c) == nil &&
			len(c.Plans) == 1 && c.Plans[0].Status == catalog.Draft {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	for !strings.Contains(e.stderr.String(), "already seen") {
		if time.Now().After(deadline) {
			t.Fatalf("duplicate not reported; log:\n%s", e.stderr)
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	if code := <-exited; code != 0 {
		t.Fatalf("exit %d: %s", code, e.stderr)
	}
	log := e.stderr.String()
	for _, want := range []string{"workers=2", "maxUsd=3", "inbox=" + inbox, "draft ready", "stopped"} {
		if !strings.Contains(log, want) {
			t.Errorf("log lacks %s", want)
		}
	}
}

func TestUsageAndConfigErrors(t *testing.T) {
	e := newEnv(t, tracer)
	bad := filepath.Join(e.dir, "bad.yaml")
	if err := os.WriteFile(bad, []byte("maxUSD: 3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		args []string
		code int
		want string
	}{
		{[]string{"--nope"}, 2, "unknown flag"},
		{[]string{"trace"}, 2, "accepts 1 arg"},
		{[]string{"plans", "approve"}, 2, "accepts 1 arg"},
		{[]string{"plans", "list", "extra"}, 2, "accepts 0 arg"},
		{[]string{"watch", "--workers", "0"}, 2, "workers must be 1–8"},
		{[]string{"trace", "notes.txt"}, 2, "is not a .png"},
		{[]string{"trace", filepath.Join(e.dir, "missing.png")}, 1, "no such file"},
		{[]string{"plans", "list", "--config", filepath.Join(e.dir, "absent.yaml")}, 1, "config file not found"},
		{[]string{"watch", "--config", bad}, 1, "field maxUSD not found"},
		{[]string{"plans", "list", "--output", filepath.Join(bad, "x")}, 1, "create output directory"},
		{[]string{"trace", "x.png", "--output", filepath.Join(bad, "x")}, 1, "create output directory"},
		{[]string{"watch", "--output", filepath.Join(bad, "x")}, 1, "create output directory"},
		{[]string{"watch", "--inbox", filepath.Join(bad, "in")}, 1, "create inbox"},
	}
	for _, c := range cases {
		t.Run(strings.Join(c.args, " "), func(t *testing.T) {
			e.stderr = &syncBuffer{}
			if code := e.run(c.args...); code != c.code || !strings.Contains(e.stderr.String(), c.want) {
				t.Fatalf("exit %d, want %d; stderr %s", code, c.code, e.stderr)
			}
		})
	}
	e.stderr = &syncBuffer{}
	if code := e.run("--version"); code != 0 || !strings.Contains(e.stdout.String(), "devel") {
		t.Fatalf("%d %s", code, e.stdout)
	}
}

func TestNodeValidatorPrerequisites(t *testing.T) {
	e := newEnv(t, tracer)
	app := e.app()
	app.Validator = nil
	e.vars["SPACEWEAVE_NODE"] = "no-such-node-binary"
	if code := Execute(context.Background(), app, []string{"watch"}); code != 1 || !strings.Contains(e.stderr.String(), "node is needed") {
		t.Fatalf("%d %s", code, e.stderr)
	}
	e.stderr = &syncBuffer{}
	app.Stderr = e.stderr
	e.vars["SPACEWEAVE_NODE"] = "go" // any executable on PATH
	if code := Execute(context.Background(), app, []string{"trace", "a.png"}); code != 1 || !strings.Contains(e.stderr.String(), "validator script") {
		t.Fatalf("%d %s", code, e.stderr)
	}
	cfg := filepath.Join(e.dir, "c.yaml")
	if err := os.WriteFile(cfg, []byte("validator: "+cfg+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(e.dir, "generated")
	if _, err := catalog.Open(store); err != nil {
		t.Fatal(err)
	}
	if code := Execute(context.Background(), app, []string{"plans", "list", "--config", cfg}); code != 0 {
		t.Fatal("review commands don't need node")
	}
}
