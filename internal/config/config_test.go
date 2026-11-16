package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func env(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
}

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "spaceweave.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDefaultsWhenNoFile(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "none.yaml"), false, env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c != Default() {
		t.Fatalf("got %+v", c)
	}
	if _, err := Load(filepath.Join(t.TempDir(), "none.yaml"), true, env(nil)); !errors.Is(err, ErrMissing) {
		t.Fatalf("a named config file must exist: %v", err)
	}
	if _, err := Load(t.TempDir(), false, env(nil)); err == nil || errors.Is(err, ErrMissing) {
		t.Fatalf("a directory is a read error: %v", err)
	}
}

func TestEmptyFileIsAllDefaults(t *testing.T) {
	c, err := Load(write(t, ""), true, env(nil))
	if err != nil || c != Default() {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestFileThenEnvironment(t *testing.T) {
	p := write(t, `
inbox: plans/in
workers: 2
effort: xhigh
limits:
  maxUsd: 4.5
  timeout: 10m
prices:
  input: 3
`)
	c, err := Load(p, true, env(map[string]string{"SPACEWEAVE_INBOX": "/drop", "SPACEWEAVE_MAX_USD": "2", "SPACEWEAVE_WORKERS": "3", "SPACEWEAVE_MODEL": "claude-sonnet-5-5"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Inbox != "/drop" || c.Workers != 3 || c.Limits.MaxUSD != 2 || c.Model != "claude-sonnet-5-5" {
		t.Fatalf("environment wins over the file: %+v", c)
	}
	if c.Effort != "xhigh" || c.Limits.Timeout != 10*time.Minute || c.Prices.Input != 3 || c.Prices.Output != 20 || c.Limits.MaxTurns != 120 {
		t.Fatalf("file values merge over defaults: %+v", c)
	}
}

func TestStrictFile(t *testing.T) {
	_, err := Load(write(t, "limits:\n  maxUSD: 100\n"), true, env(nil))
	if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "maxUSD") {
		t.Fatalf("a misspelt key must be rejected, not ignored: %v", err)
	}
	if _, err := Load(write(t, "workers: [1"), true, env(nil)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad YAML: %v", err)
	}
}

func TestBadEnvironment(t *testing.T) {
	for _, k := range []string{"SPACEWEAVE_WORKERS", "SPACEWEAVE_MAX_USD"} {
		if _, err := Load("none", false, env(map[string]string{k: "lots"})); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", k, err)
		}
	}
}

func TestValidate(t *testing.T) {
	cases := map[string]func(*Config){
		"missing path": func(c *Config) { c.Validator = "" },
		"workers":      func(c *Config) { c.Workers = 0 },
		"effort":       func(c *Config) { c.Effort = "extreme" },
		"limits":       func(c *Config) { c.Limits.MaxUSD = 0 },
		"prices":       func(c *Config) { c.Prices.CacheRead = -1 },
		"settle":       func(c *Config) { c.Settle = -time.Second },
	}
	for name, breakIt := range cases {
		c := Default()
		breakIt(&c)
		if err := c.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if len(Env) != 7 {
		t.Fatal("Env documents every variable applyEnv reads")
	}
}

// The example file documents every key with its default; this keeps it
// from drifting when a default changes.
func TestExampleFileIsTheDefaults(t *testing.T) {
	c, err := Load(filepath.Join("..", "..", "spaceweave.example.yaml"), true, env(nil))
	if err != nil || c != Default() {
		t.Fatalf("spaceweave.example.yaml: %+v %v", c, err)
	}
}
