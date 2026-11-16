// Package acceptance drives the built spaceweave binary end to end: a real
// inbox folder, the real node validator, and a fake Messages API standing in
// for Claude. The binary is built with coverage on, and its coverage lands
// in SPACEWEAVE_COVERDIR for `make cover` to merge with the unit tests'.
package acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/weaveplatform/spaceweave/internal/catalog"
	"github.com/weaveplatform/spaceweave/internal/fakeclaude"
)

var (
	binary   string
	repoRoot string
	coverDir string
)

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	repoRoot = filepath.Join(wd, "..", "..")
	tmp, err := os.MkdirTemp("", "spaceweave-acceptance-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmp) //nolint:errcheck // temp dir
	coverDir = os.Getenv("SPACEWEAVE_COVERDIR")
	if coverDir == "" {
		// The binary warns on every run without somewhere to write coverage.
		coverDir = filepath.Join(tmp, "cover")
		if err := os.Mkdir(coverDir, 0o750); err != nil {
			panic(err)
		}
	}
	binary = filepath.Join(tmp, "spaceweave")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	// atomic, because the unit tests run with -race and `go tool covdata`
	// won't merge counters recorded in different modes.
	build := exec.Command("go", "build", "-cover", "-covermode=atomic", "-coverpkg=github.com/weaveplatform/spaceweave/...",
		"-o", binary, "./cmd/spaceweave")
	build.Dir = repoRoot
	build.Env = append(os.Environ(), "GOWORK=off")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build: %v\n%s", err, out)
		return 1
	}
	return m.Run()
}

// lockedBuffer collects a running process's output for reading mid-run.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

type site struct {
	t      *testing.T
	dir    string
	inbox  string
	output string
	env    []string
	api    *fakeclaude.Server
}

func newSite(t *testing.T, script fakeclaude.Script) *site {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is needed for the plan validator")
	}
	dir := t.TempDir()
	s := &site{t: t, dir: dir, inbox: filepath.Join(dir, "inbox"), output: filepath.Join(dir, "generated")}
	if err := os.MkdirAll(s.inbox, 0o750); err != nil {
		t.Fatal(err)
	}
	s.api = fakeclaude.New(script)
	t.Cleanup(s.api.Close)
	cfg := fmt.Sprintf("inbox: %s\noutput: %s\nvalidator: %s\nsettle: 100ms\nlimits:\n  maxTurns: 20\n  maxSubmits: 4\n  maxUsd: 5\n  timeout: 1m\n",
		s.inbox, s.output, filepath.Join(repoRoot, "web", "floorplan-iso", "tools", "validate.mjs"))
	if err := os.WriteFile(filepath.Join(dir, "spaceweave.yaml"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	s.env = append(os.Environ(),
		"ANTHROPIC_BASE_URL="+s.api.URL, "ANTHROPIC_API_KEY=test-key", "GOCOVERDIR="+coverDir)
	return s
}

func (s *site) command(args ...string) *exec.Cmd {
	cmd := exec.Command(binary, args...) //nolint:gosec // the binary under test
	cmd.Dir = s.dir                      // so spaceweave.yaml is the default config
	cmd.Env = s.env
	return cmd
}

func (s *site) run(args ...string) (string, string, int) {
	s.t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := s.command(args...)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		exit, ok := err.(*exec.ExitError) //nolint:errorlint // Run returns it unwrapped
		if !ok {
			s.t.Fatal(err)
		}
		code = exit.ExitCode()
	}
	return stdout.String(), stderr.String(), code
}

func (s *site) plans() []catalog.Entry {
	s.t.Helper()
	out, errOut, code := s.run("plans", "list", "--json")
	if code != 0 {
		s.t.Fatalf("plans list: %d %s", code, errOut)
	}
	var ps []catalog.Entry
	if err := json.Unmarshal([]byte(out), &ps); err != nil {
		s.t.Fatal(err, out)
	}
	return ps
}

func (s *site) waitFor(what string, cond func([]catalog.Entry) bool, log *lockedBuffer) []catalog.Entry {
	s.t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		var c catalog.Catalog
		if b, err := os.ReadFile(filepath.Join(s.output, "catalog.json")); err == nil && json.Unmarshal(b, &c) == nil && cond(c.Plans) {
			return c.Plans
		}
		if time.Now().After(deadline) {
			s.t.Fatalf("timed out waiting for %s; daemon log:\n%s", what, log)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// floorPlan draws a 10 m × 5 m room at 20 px/m with 4 px walls.
func floorPlan(t *testing.T, path string, mark uint8) {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, 200, 100))
	for y := range 100 {
		for x := range 200 {
			c := color.Gray{255}
			if x < 4 || x >= 196 || y < 4 || y >= 96 {
				c = color.Gray{0}
			}
			img.SetGray(x, y, c)
		}
	}
	img.SetGray(100, 50, color.Gray{mark}) // makes otherwise identical plans distinct
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func roomTrace() map[string]any {
	wall := func(id string, x0, y0, x1, y1 float64, openings ...any) map[string]any {
		w := map[string]any{"id": id, "kind": "exterior", "from": []float64{x0, y0}, "to": []float64{x1, y1}}
		if len(openings) > 0 {
			w["openings"] = openings
		}
		return w
	}
	return map[string]any{
		"title":       "Studio",
		"calibration": map[string]any{"pxPerMetre": []float64{20, 20}, "evidence": "door leaf 18 px = 0.9 m"},
		"thickness":   map[string]any{"exterior": 0.2, "interior": 0.1, "glass": 0.05},
		"entrance":    []float64{2, 50},
		"walls": []any{
			wall("n", 2, 2, 198, 2, map[string]any{"a": 60, "b": 120, "kind": "window"}),
			wall("s", 2, 98, 198, 98),
			wall("w", 2, 2, 2, 98, map[string]any{"a": 41, "b": 59, "kind": "door", "swing": "E", "hinge": "a"}),
			wall("e", 198, 2, 198, 98),
		},
		"rooms": []any{map[string]any{
			"id": "studio", "name": "Studio", "zone": "work", "floor": "carpet",
			"polygon": [][]float64{{2, 2}, {198, 2}, {198, 98}, {2, 98}},
		}},
		"furniture": []any{
			map[string]any{"type": "desk", "x": 100, "y": 30, "w": 1.6, "d": 0.8},
			map[string]any{"type": "chair", "x": 100, "y": 45, "facing": 270},
			map[string]any{"type": "plant", "x": 185, "y": 85},
		},
	}
}

// claude scripts a model that submits the trace and finishes, and records
// the opening message of each run so tests can see reviewer feedback arrive.
type claude struct {
	mu       sync.Mutex
	openings []string
}

func (c *claude) script(_ int, req fakeclaude.Request) fakeclaude.Reply {
	use := func(name string, in any) fakeclaude.Reply {
		return fakeclaude.Reply{
			Content: []fakeclaude.Block{{Name: name, Input: in}}, StopReason: "tool_use",
			Usage: fakeclaude.Usage{Input: 50000, Output: 5000, CacheRead: 100000},
		}
	}
	if len(req.Messages) == 1 {
		c.mu.Lock()
		c.openings = append(c.openings, string(req.Messages[0]))
		c.mu.Unlock()
		return use("submit_trace", map[string]any{"trace": roomTrace()})
	}
	return use("finish", map[string]any{"confidence": 0.9, "summary": "A one-room studio", "caveats": []string{"Furniture is indicative"}})
}

func (c *claude) opening(i int) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if i >= len(c.openings) {
		return ""
	}
	return c.openings[i]
}

func TestWatchApproveReject(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stopping the daemon needs an interrupt signal")
	}
	model := &claude{}
	s := newSite(t, model.script)
	var log lockedBuffer
	daemon := s.command("watch", "--workers", "2")
	daemon.Stdout, daemon.Stderr = &log, &log
	if err := daemon.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- daemon.Wait() }()
	stopped := false
	defer func() {
		if !stopped {
			_ = daemon.Process.Kill()
			<-exited
		}
	}()

	floorPlan(t, filepath.Join(s.inbox, "Studio Flat.png"), 1)

	ps := s.waitFor("a draft", func(ps []catalog.Entry) bool {
		return len(ps) == 1 && ps[0].Status == catalog.Draft
	}, &log)
	id := ps[0].ID
	dir := filepath.Join(s.output, id)

	// The published plan passes the site's own validator.
	out, err := exec.Command("node", filepath.Join(repoRoot, "web", "floorplan-iso", "tools", "validate.mjs"),
		filepath.Join(dir, "plan.json")).Output()
	if err != nil || strings.TrimSpace(string(out)) != `{"ok":true,"errors":[]}` {
		t.Fatalf("validate: %v %s", err, out)
	}
	var p struct {
		Source struct {
			URL    string `json:"url"`
			Credit string `json:"credit"`
			Title  string `json:"title"`
		} `json:"SOURCE"` //nolint:tagliatelle // the site's own key
		Width float64 `json:"WIDTH"` //nolint:tagliatelle // the site's own key
	}
	b, _ := os.ReadFile(filepath.Join(dir, "plan.json"))
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	if p.Source.URL != "generated/"+id+"/source.png" || p.Source.Credit != "Uploaded Studio Flat.png" || p.Width != 9.8 {
		t.Fatalf("plan.json: %s", b)
	}
	var report map[string]any
	b, _ = os.ReadFile(filepath.Join(dir, "report.json"))
	if err := json.Unmarshal(b, &report); err != nil || report["status"] != "draft" || report["confidence"] != 0.9 {
		t.Fatalf("report.json: %s", b)
	}

	// Approve, then send back with a note: the daemon picks the re-queued
	// plan up on its own and the note reaches the model.
	if _, errOut, code := s.run("plans", "approve", id); code != 0 {
		t.Fatal(errOut)
	}
	if got := s.plans()[0].Status; got != catalog.Approved {
		t.Fatal(got)
	}
	if _, errOut, code := s.run("plans", "reject", id, "--note", "The door is on the wrong wall"); code != 0 {
		t.Fatal(errOut)
	}
	s.waitFor("the second attempt", func(ps []catalog.Entry) bool {
		return ps[0].Status == catalog.Draft && ps[0].Attempts == 2
	}, &log)
	second := model.opening(1)
	if !strings.Contains(second, "The door is on the wrong wall") || !strings.Contains(second, `\"title\":\"Studio\"`) {
		t.Fatalf("feedback didn't reach the model: %.300s", second)
	}

	// The same image again is recognised; a different one is a new plan.
	floorPlan(t, filepath.Join(s.inbox, "copy of studio.png"), 1)
	floorPlan(t, filepath.Join(s.inbox, "annex.png"), 2)
	s.waitFor("the annex", func(ps []catalog.Entry) bool {
		return len(ps) == 2 && ps[1].Status == catalog.Draft
	}, &log)

	if err := daemon.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	stopped = true
	select {
	case err := <-exited:
		if err != nil {
			t.Fatalf("daemon exit: %v\n%s", err, &log)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("daemon didn't stop on interrupt")
	}
	for _, want := range []string{"already seen", "draft ready", "stopped"} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("daemon log lacks %q:\n%s", want, &log)
		}
	}
	if len(s.plans()) != 2 {
		t.Fatal("the copy was traced again")
	}
}

func TestOneOffTraceAndFailure(t *testing.T) {
	refuse := func(int, fakeclaude.Request) fakeclaude.Reply {
		return fakeclaude.Reply{StopReason: "refusal", Category: "cyber", Usage: fakeclaude.Usage{Input: 1000}}
	}
	s := newSite(t, refuse)
	img := filepath.Join(s.dir, "plan.png")
	floorPlan(t, img, 3)
	out, errOut, code := s.run("trace", img)
	if code != 1 || !strings.Contains(errOut, "trace failed") || !strings.Contains(errOut, "the model declined (cyber)") {
		t.Fatalf("%d\n%s\n%s", code, out, errOut)
	}
	ps := s.plans()
	if ps[0].Status != catalog.Failed {
		t.Fatalf("%+v", ps)
	}
	if _, errOut, code := s.run("plans", "retry", ps[0].ID); code != 0 {
		t.Fatal(errOut)
	}
	if _, _, code := s.run("plans", "approve", ps[0].ID); code != 1 {
		t.Fatal("a queued plan can't be approved")
	}
	if _, errOut, code := s.run("trace"); code != 2 || !strings.Contains(errOut, "accepts 1 arg") {
		t.Fatalf("%d %s", code, errOut)
	}
}

// TestLive traces one image with the real API. It costs money, so it runs
// only with SPACEWEAVE_LIVE=1 and an image in SPACEWEAVE_LIVE_IMAGE.
func TestLive(t *testing.T) {
	if os.Getenv("SPACEWEAVE_LIVE") != "1" {
		t.Skip("set SPACEWEAVE_LIVE=1 (and ANTHROPIC_API_KEY) to trace a real image")
	}
	img := os.Getenv("SPACEWEAVE_LIVE_IMAGE")
	if img == "" {
		t.Fatal("set SPACEWEAVE_LIVE_IMAGE to a floor-plan image")
	}
	s := newSite(t, func(int, fakeclaude.Request) fakeclaude.Reply { return fakeclaude.Reply{Status: 500} })
	env := s.env[:0:0]
	for _, kv := range s.env {
		if !strings.HasPrefix(kv, "ANTHROPIC_BASE_URL=") && !strings.HasPrefix(kv, "ANTHROPIC_API_KEY=test") {
			env = append(env, kv)
		}
	}
	s.env = env
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "trace", img, "--max-usd", "5") //nolint:gosec // the binary under test
	cmd.Dir, cmd.Env = s.dir, s.env
	out, err := cmd.CombinedOutput()
	t.Logf("%s", out)
	if err != nil {
		t.Fatal(err)
	}
	ps := s.plans()
	if ps[0].Status != catalog.Draft {
		t.Fatalf("%+v", ps[0])
	}
	t.Logf("confidence %.2f, cost $%.2f, files in %s", *ps[0].Confidence, ps[0].CostUSD, filepath.Join(s.output, ps[0].ID))
}
