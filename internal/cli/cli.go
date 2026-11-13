// Package cli is the spaceweave command line: the inbox daemon, one-off
// traces, and the review commands that approve or send back drafts.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"sync"
	"text/tabwriter"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
	"github.com/spf13/cobra"

	"github.com/weaveplatform/spaceweave/internal/buildinfo"
	"github.com/weaveplatform/spaceweave/internal/catalog"
	"github.com/weaveplatform/spaceweave/internal/config"
	"github.com/weaveplatform/spaceweave/internal/inbox"
	"github.com/weaveplatform/spaceweave/internal/plan"
	"github.com/weaveplatform/spaceweave/internal/trace"
	"github.com/weaveplatform/spaceweave/internal/worker"
)

// Errors. ErrUsage is a command used wrongly; Execute exits 2 for it.
var (
	ErrUsage       = errors.New("usage")
	ErrTraceFailed = errors.New("trace failed")
)

// App is what the commands need from outside. Tests replace the pieces that
// reach the network or the filesystem's node install.
type App struct {
	Stdout, Stderr io.Writer
	Getenv         func(string) (string, bool)
	// ClientOptions are added to the Anthropic client's; tests point it at
	// a fake API here.
	ClientOptions []option.RequestOption
	// Validator, when set, replaces the node validator.
	Validator plan.Validator
}

type flags struct {
	config  string
	inbox   string
	output  string
	model   string
	maxUSD  float64
	workers int
	verbose bool
}

// Execute runs the command line and returns the process exit code.
func Execute(ctx context.Context, app App, args []string) int {
	//nolint:contextcheck // cobra hands ctx to each command through ExecuteContext
	root := New(app)
	root.SetArgs(args)
	err := root.ExecuteContext(ctx)
	switch {
	case err == nil:
		return 0
	case errors.Is(err, ErrUsage):
		_, _ = fmt.Fprintln(app.Stderr, "error:", err)
		return 2
	default:
		_, _ = fmt.Fprintln(app.Stderr, "error:", err)
		return 1
	}
}

// New builds the command tree.
func New(app App) *cobra.Command {
	f := &flags{}
	root := &cobra.Command{
		Use:   "spaceweave",
		Short: "Trace floor-plan images into the isometric microsite",
		Long: "spaceweave watches a folder for floor-plan images, has Claude trace each one into a plan, " +
			"and publishes it to the microsite as a draft for review.\n\n" +
			"Images are sent to the Claude API. The API key comes from ANTHROPIC_API_KEY or an `ant auth login` profile.",
		Version:       buildinfo.Version(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(app.Stdout)
	root.SetErr(app.Stderr)
	root.SetFlagErrorFunc(
		func(_ *cobra.Command, err error) error { return fmt.Errorf("%w: %w", ErrUsage, err) },
	)
	p := root.PersistentFlags()
	p.StringVar(&f.config, "config", "", "config file (default "+config.DefaultPath+" if present)")
	p.StringVar(&f.inbox, "inbox", "", "folder watched for new images")
	p.StringVar(&f.output, "output", "", "the microsite's generated/ folder")
	p.StringVar(&f.model, "model", "", "Claude model")
	p.Float64Var(&f.maxUSD, "max-usd", 0, "spending cap per plan, US dollars")
	p.IntVar(&f.workers, "workers", 0, "plans traced at once")
	p.BoolVarP(&f.verbose, "verbose", "v", false, "debug logging")

	root.AddCommand(watchCmd(app, f), traceCmd(app, f), plansCmd(app, f))
	return root
}

func usageArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(n)(cmd, args); err != nil {
			return fmt.Errorf("%w: %w", ErrUsage, err)
		}
		return nil
	}
}

// load resolves the configuration: defaults, then the file, then the
// environment, then flags.
func (f *flags) load(cmd *cobra.Command, app App) (config.Config, error) {
	path, required := f.config, true
	if path == "" {
		path, required = config.DefaultPath, false
	}
	c, err := config.Load(path, required, app.Getenv)
	if err != nil {
		return c, err //nolint:wrapcheck // config errors name the file
	}
	set := cmd.Flags().Changed
	if set("inbox") {
		c.Inbox = f.inbox
	}
	if set("output") {
		c.Output = f.output
	}
	if set("model") {
		c.Model = f.model
	}
	if set("max-usd") {
		c.Limits.MaxUSD = f.maxUSD
	}
	if set("workers") {
		c.Workers = f.workers
	}
	if err := c.Validate(); err != nil {
		return c, fmt.Errorf("%w: %w", ErrUsage, err)
	}
	return c, nil
}

func (f *flags) logger(app App) *slog.Logger {
	level := slog.LevelInfo
	if f.verbose {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewTextHandler(app.Stderr, &slog.HandlerOptions{Level: level}))
}

func openStore(c config.Config) (*catalog.Store, error) {
	s, err := catalog.Open(c.Output)
	if err != nil {
		return nil, err //nolint:wrapcheck // names itself
	}
	return s, nil
}

// newWorker wires the tracer to the configuration. It checks node and the
// validator script up front: without them every trace would run to its
// first submission, spend money, and then fail.
func newWorker(
	app App,
	c config.Config,
	store *catalog.Store,
	log *slog.Logger,
) (*worker.Worker, error) {
	v := app.Validator
	if v == nil {
		if _, err := exec.LookPath(c.Node); err != nil {
			return nil, fmt.Errorf("node is needed to validate plans: %w", err)
		}
		if _, err := os.Stat(c.Validator); err != nil {
			return nil, fmt.Errorf(
				"validator script: %w (run from the repository root, or set validator)",
				err,
			)
		}
		v = plan.NodeValidator{Node: c.Node, Script: c.Validator}
	}
	return &worker.Worker{
		Store: store,
		Log:   log,
		Options: trace.Options{
			Client:    anthropic.NewClient(app.ClientOptions...),
			Model:     c.Model,
			Effort:    c.Effort,
			Limits:    c.Limits,
			Prices:    c.Prices,
			Validator: v,
		},
	}, nil
}

func watchCmd(app App, f *flags) *cobra.Command {
	return &cobra.Command{
		Use:   "watch",
		Short: "Watch the inbox and trace each new image",
		Long: "Watches the inbox folder. Each new image is copied into the output folder, queued, " +
			"traced and published as a draft. Runs until interrupted.",
		Args: usageArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := f.load(cmd, app)
			if err != nil {
				return err
			}
			log := f.logger(app)
			store, err := openStore(c)
			if err != nil {
				return err
			}
			w, err := newWorker(app, c, store, log)
			if err != nil {
				return err
			}
			wake := make(chan struct{}, 1)
			watcher := &inbox.Watcher{
				Dir: c.Inbox, Settle: c.Settle, Log: log,
				Ingest: func(path string) error {
					e, created, err := store.Ingest(path)
					if err != nil {
						return err //nolint:wrapcheck // logged by the watcher with the file name
					}
					if !created {
						log.Info("already seen", "file", path, "plan", e.ID, "status", e.Status)
						return nil
					}
					log.Info("queued", "file", path, "plan", e.ID)
					select {
					case wake <- struct{}{}:
					default:
					}
					return nil
				},
			}
			log.Info("watching", "inbox", c.Inbox, "output", c.Output, "model", c.Model,
				"workers", c.Workers, "maxUsd", c.Limits.MaxUSD)

			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()
			var wg sync.WaitGroup
			var watchErr, workErr error
			wg.Go(func() {
				watchErr = watcher.Run(ctx)
				cancel()
			})
			wg.Go(func() {
				workErr = w.Run(ctx, c.Workers, wake)
				cancel()
			})
			wg.Wait()
			log.Info("stopped")
			return errors.Join(watchErr, workErr)
		},
	}
}

func traceCmd(app App, f *flags) *cobra.Command {
	return &cobra.Command{
		Use:   "trace <image>",
		Short: "Trace one image now, without the watcher",
		Long: "Adds the image to the output folder and traces it straight away. " +
			"An image already in the catalogue isn't traced again; use `plans reject` or `plans retry` for that.",
		Args: usageArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := f.load(cmd, app)
			if err != nil {
				return err
			}
			store, err := openStore(c)
			if err != nil {
				return err
			}
			w, err := newWorker(app, c, store, f.logger(app))
			if err != nil {
				return err
			}
			if !inbox.Eligible(args[0]) {
				return fmt.Errorf(
					"%w: %s is not a .png, .jpg, .jpeg or .webp file",
					ErrUsage,
					args[0],
				)
			}
			e, created, err := store.Ingest(args[0])
			if err != nil {
				return err //nolint:wrapcheck // names the file
			}
			if !created {
				_, _ = fmt.Fprintf(
					app.Stdout,
					"%s is already plan %s (%s)\n",
					args[0],
					e.ID,
					e.Status,
				)
				return nil
			}
			if err := store.Transition(
				e.ID,
				catalog.Tracing,
				func(x *catalog.Entry) { x.Attempts++ },
			); err != nil {
				return err //nolint:wrapcheck // names itself
			}
			e, err = store.Get(e.ID)
			if err != nil {
				return err //nolint:wrapcheck // names itself
			}
			if err := w.Process(cmd.Context(), e); err != nil {
				return err //nolint:wrapcheck // names itself
			}
			e, err = store.Get(e.ID)
			if err != nil {
				return err //nolint:wrapcheck // names itself
			}
			printEntry(app.Stdout, e, store)
			if e.Status == catalog.Failed {
				return fmt.Errorf("%w: %s", ErrTraceFailed, e.Error)
			}
			return nil
		},
	}
}

func printEntry(w io.Writer, e catalog.Entry, store *catalog.Store) {
	_, _ = fmt.Fprintf(
		w,
		"%s  %s  confidence %s  cost $%.2f\n  %s\n",
		e.ID,
		e.Status,
		confidence(e),
		e.CostUSD,
		store.PlanDir(e.ID),
	)
}

func confidence(e catalog.Entry) string {
	if e.Confidence == nil {
		return "-"
	}
	return strconv.FormatFloat(*e.Confidence, 'f', 2, 64)
}

func plansCmd(app App, f *flags) *cobra.Command {
	cmd := &cobra.Command{Use: "plans", Short: "Review traced plans"}
	var asJSON bool
	list := &cobra.Command{
		Use:   "list",
		Short: "List plans with their status, confidence and cost",
		Args:  usageArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := f.store(cmd, app)
			if err != nil {
				return err
			}
			c, err := store.Load()
			if err != nil {
				return err //nolint:wrapcheck // names itself
			}
			if asJSON {
				enc := json.NewEncoder(app.Stdout)
				enc.SetIndent("", " ")
				return enc.Encode(c.Plans) //nolint:wrapcheck // a write to stdout
			}
			tw := tabwriter.NewWriter(app.Stdout, 0, 4, 2, ' ', 0)
			_, _ = fmt.Fprintln(tw, "ID\tSTATUS\tCONFIDENCE\tCOST\tATTEMPTS\tTITLE")
			for _, e := range c.Plans {
				_, _ = fmt.Fprintf(
					tw,
					"%s\t%s\t%s\t$%.2f\t%d\t%s\n",
					e.ID,
					e.Status,
					confidence(e),
					e.CostUSD,
					e.Attempts,
					e.Title,
				)
			}
			return tw.Flush() //nolint:wrapcheck // a write to stdout
		},
	}
	list.Flags().BoolVar(&asJSON, "json", false, "print the catalogue entries as JSON")

	approve := &cobra.Command{
		Use:   "approve <id>",
		Short: "Approve a draft; the site drops its Draft badge",
		Args:  usageArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return f.move(cmd, app, args[0], []catalog.Status{catalog.Draft}, catalog.Approved, nil)
		},
	}

	var note string
	reject := &cobra.Command{
		Use:   "reject <id>",
		Short: "Send a draft (or approved plan) back for another pass",
		Long: "Re-queues the plan. The note goes to the tracer as reviewer feedback, " +
			"along with the rejected trace to correct.",
		Args: usageArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return f.move(
				cmd,
				app,
				args[0],
				[]catalog.Status{catalog.Draft, catalog.Approved},
				catalog.Queued,
				func(e *catalog.Entry) { e.Note = note },
			)
		},
	}
	reject.Flags().StringVar(&note, "note", "", "what to fix")

	retry := &cobra.Command{
		Use:   "retry <id>",
		Short: "Re-queue a failed plan",
		Args:  usageArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return f.move(cmd, app, args[0], []catalog.Status{catalog.Failed}, catalog.Queued,
				func(e *catalog.Entry) { e.Error = "" })
		},
	}
	cmd.AddCommand(list, approve, reject, retry)
	return cmd
}

func (f *flags) store(cmd *cobra.Command, app App) (*catalog.Store, error) {
	c, err := f.load(cmd, app)
	if err != nil {
		return nil, err
	}
	return openStore(c)
}

// move changes a plan's status. from narrows the catalogue's own rules to
// what the command means: reject and retry both re-queue, but rejecting a
// failure or retrying a draft is a mistake worth saying so.
func (f *flags) move(
	cmd *cobra.Command,
	app App,
	id string,
	from []catalog.Status,
	to catalog.Status,
	mutate func(*catalog.Entry),
) error {
	store, err := f.store(cmd, app)
	if err != nil {
		return err
	}
	e, err := store.Get(id)
	if err != nil {
		return err //nolint:wrapcheck // names the plan
	}
	if !slices.Contains(from, e.Status) {
		return fmt.Errorf("%w: %s is %s", catalog.ErrTransition, id, e.Status)
	}
	if err := store.Transition(id, to, mutate); err != nil {
		return err //nolint:wrapcheck // names itself
	}
	_, _ = fmt.Fprintf(app.Stdout, "%s: %s → %s\n", id, e.Status, to)
	return nil
}
