package plan

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ErrValidator is returned when the validator can't give a verdict, as
// opposed to a verdict that the plan is invalid.
var ErrValidator = errors.New("plan validator failed")

// Validator checks a plan.json against the site's invariants.
type Validator interface {
	Validate(ctx context.Context, planJSON []byte) ([]string, error)
}

// NodeValidator runs web/floorplan-iso/tools/validate.mjs, so a plan the
// worker publishes passes exactly the checks the site's own tests apply.
type NodeValidator struct {
	Node   string // the node executable
	Script string // path to tools/validate.mjs
}

type verdict struct {
	OK     bool     `json:"ok"`
	Errors []string `json:"errors"`
}

// Validate writes the plan to a temporary file and runs the script on it.
func (v NodeValidator) Validate(ctx context.Context, planJSON []byte) ([]string, error) {
	dir, err := os.MkdirTemp("", "spaceweave-validate-")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrValidator, err)
	}
	defer os.RemoveAll(dir) //nolint:errcheck // best-effort cleanup of a temp dir
	path := filepath.Join(dir, "plan.json")
	if err := os.WriteFile(path, planJSON, 0o600); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrValidator, err)
	}

	var stdout, stderr bytes.Buffer
	//nolint:gosec // node and script come from the worker's config, not the trace
	cmd := exec.CommandContext(ctx, v.Node, v.Script, path)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf(
			"%w: %s %s: %w: %s",
			ErrValidator,
			v.Node,
			v.Script,
			err,
			bytes.TrimSpace(stderr.Bytes()),
		)
	}
	var out verdict
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return nil, fmt.Errorf("%w: unreadable verdict %q: %w", ErrValidator, stdout.String(), err)
	}
	if !out.OK && len(out.Errors) == 0 {
		return nil, fmt.Errorf("%w: invalid with no reasons", ErrValidator)
	}
	return out.Errors, nil
}
