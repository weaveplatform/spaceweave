// Package config loads spaceweave.yaml. Precedence is flag > environment >
// file > default; the CLI applies flags last, on top of what Load returns.
// The file is strict: an unknown key is an error, because a misspelt limit
// (maxUSD for maxUsd, say) would otherwise be ignored and the default
// spending cap silently apply.
//
// The Anthropic API key is deliberately not a setting. The SDK reads it from
// ANTHROPIC_API_KEY or an `ant auth login` profile, so it never sits in a
// file that might be committed.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// Config is the worker's configuration.
type Config struct {
	Inbox     string `yaml:"inbox"`     // folder watched for new plan images
	Output    string `yaml:"output"`    // the microsite's generated/ folder
	Node      string `yaml:"node"`      // node executable for the validator
	Validator string `yaml:"validator"` // web/floorplan-iso/tools/validate.mjs
	Workers   int    `yaml:"workers"`   // plans traced at once
	Model     string `yaml:"model"`
	Effort    string `yaml:"effort"`
	Limits    Limits `yaml:"limits"`
	Prices    Prices `yaml:"prices"`
	// Settle is how long an inbox file's size must stay unchanged before it
	// is picked up, so a file still being copied isn't read half-written.
	Settle time.Duration `yaml:"settle"`
	Serve  Serve         `yaml:"serve"`
}

// Serve configures `spaceweave serve`.
type Serve struct {
	Addr string `yaml:"addr"` // listen address
	Site string `yaml:"site"` // the microsite's folder
	// Hosts are extra host names the server answers to. Only loopback names
	// are accepted by default, because any page in the browser can make
	// requests to a local server, and a DNS-rebound one can read the replies.
	Hosts []string `yaml:"hosts"`
}

// Limits stop a trace that isn't converging. Whichever is hit first ends
// the run; a valid trace by then is still published, marked as stopped.
type Limits struct {
	MaxTurns   int           `yaml:"maxTurns"`
	MaxSubmits int           `yaml:"maxSubmits"`
	MaxUSD     float64       `yaml:"maxUsd"`
	Timeout    time.Duration `yaml:"timeout"`
}

// Prices are US dollars per million tokens, for the spending cap. They
// default to Claude Opus 5.5's list prices; set them when using another model.
type Prices struct {
	Input      float64 `yaml:"input"`
	Output     float64 `yaml:"output"`
	CacheRead  float64 `yaml:"cacheRead"`
	CacheWrite float64 `yaml:"cacheWrite"`
}

// Efforts are the accepted effort levels.
var Efforts = []string{"low", "medium", "high", "xhigh", "max"}

// Errors.
var (
	ErrInvalid = errors.New("invalid configuration")
	ErrMissing = errors.New("config file not found")
)

// Default is the configuration with no file, environment or flags.
func Default() Config {
	return Config{
		Inbox:     "floorplans/inbox",
		Output:    "web/floorplan-iso/generated",
		Node:      "node",
		Validator: "web/floorplan-iso/tools/validate.mjs",
		Workers:   1,
		Model:     "claude-opus-5-5",
		Effort:    "high",
		Limits:    Limits{MaxTurns: 120, MaxSubmits: 12, MaxUSD: 15, Timeout: 45 * time.Minute},
		Prices:    Prices{Input: 4, Output: 20, CacheRead: 0.2, CacheWrite: 5},
		Settle:    time.Second,
		Serve:     Serve{Addr: "127.0.0.1:8000", Site: "web/floorplan-iso"},
	}
}

// DefaultPath is read when no --config is given, if it exists.
const DefaultPath = "spaceweave.yaml"

// Load reads path over the defaults and applies the environment. A missing
// file is fine unless required (the user named it).
func Load(path string, required bool, getenv func(string) (string, bool)) (Config, error) {
	c := Default()
	b, err := os.ReadFile(path) //nolint:gosec // the user's own config path
	switch {
	case errors.Is(err, fs.ErrNotExist) && !required:
	case errors.Is(err, fs.ErrNotExist):
		return c, fmt.Errorf("%w: %s", ErrMissing, path)
	case err != nil:
		return c, fmt.Errorf("read %s: %w", path, err)
	default:
		dec := yaml.NewDecoder(bytes.NewReader(b))
		dec.KnownFields(true)
		// An empty file decodes as io.EOF; it just means "all defaults".
		if err := dec.Decode(&c); err != nil && !errors.Is(err, io.EOF) {
			return c, fmt.Errorf("%w: %s: %w", ErrInvalid, path, err)
		}
	}
	if err := c.applyEnv(getenv); err != nil {
		return c, err
	}
	return c, c.Validate()
}

// Env lists the environment variables Load reads.
var Env = map[string]string{
	"SPACEWEAVE_INBOX":   "inbox",
	"SPACEWEAVE_OUTPUT":  "output",
	"SPACEWEAVE_NODE":    "node",
	"SPACEWEAVE_MODEL":   "model",
	"SPACEWEAVE_EFFORT":  "effort",
	"SPACEWEAVE_WORKERS": "workers",
	"SPACEWEAVE_MAX_USD": "limits.maxUsd",
	"SPACEWEAVE_ADDR":    "serve.addr",
}

func (c *Config) applyEnv(getenv func(string) (string, bool)) error {
	strs := map[string]*string{
		"SPACEWEAVE_INBOX": &c.Inbox, "SPACEWEAVE_OUTPUT": &c.Output, "SPACEWEAVE_NODE": &c.Node,
		"SPACEWEAVE_MODEL": &c.Model, "SPACEWEAVE_EFFORT": &c.Effort,
		"SPACEWEAVE_ADDR": &c.Serve.Addr,
	}
	for k, p := range strs {
		if v, ok := getenv(k); ok {
			*p = v
		}
	}
	if v, ok := getenv("SPACEWEAVE_WORKERS"); ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("%w: SPACEWEAVE_WORKERS=%q: %w", ErrInvalid, v, err)
		}
		c.Workers = n
	}
	if v, ok := getenv("SPACEWEAVE_MAX_USD"); ok {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("%w: SPACEWEAVE_MAX_USD=%q: %w", ErrInvalid, v, err)
		}
		c.Limits.MaxUSD = f
	}
	return nil
}

// Validate rejects settings the worker can't run with.
func (c Config) Validate() error {
	var problems []string
	bad := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }
	if c.Inbox == "" || c.Output == "" || c.Node == "" || c.Validator == "" || c.Model == "" {
		bad("inbox, output, node, validator and model must all be set")
	}
	if c.Workers < 1 || c.Workers > 8 {
		bad("workers must be 1–8, got %d", c.Workers)
	}
	if !slices.Contains(Efforts, c.Effort) {
		bad("effort must be one of %v, got %q", Efforts, c.Effort)
	}
	l := c.Limits
	if l.MaxTurns < 1 || l.MaxSubmits < 1 || !(l.MaxUSD > 0) || l.Timeout <= 0 {
		bad("limits.maxTurns, maxSubmits, maxUsd and timeout must all be positive")
	}
	p := c.Prices
	if p.Input < 0 || p.Output < 0 || p.CacheRead < 0 || p.CacheWrite < 0 {
		bad("prices can't be negative")
	}
	if c.Settle < 0 {
		bad("settle can't be negative")
	}
	if c.Serve.Addr == "" || c.Serve.Site == "" {
		bad("serve.addr and serve.site must be set")
	}
	if len(problems) > 0 {
		return fmt.Errorf("%w: %s", ErrInvalid, strings.Join(problems, "\n"))
	}
	return nil
}
