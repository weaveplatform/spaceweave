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
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
	"github.com/spf13/cobra"

	"github.com/weaveplatform/spaceweave/internal/buildinfo"
	"github.com/weaveplatform/spaceweave/internal/catalog"
	"github.com/weaveplatform/spaceweave/internal/config"
	"github.com/weaveplatform/spaceweave/internal/inbox"
	"github.com/weaveplatform/spaceweave/internal/plan"
	"github.com/weaveplatform/spaceweave/internal/server"
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
	addr    string
	site    string
	hosts   []string
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

	root.AddCommand(watchCmd(app, f), serveCmd(app, f), traceCmd(app, f), plansCmd(app, f))
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
	if set("addr") {
		c.Serve.Addr = f.addr
	}
	if set("site") {
		c.Serve.Site = f.site
	}
	if set("allow-host") {
		c.Serve.Hosts = append(c.Serve.Hosts, f.hosts...)
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

// daemon is what watch and serve share: the inbox watcher feeding the
// workers.
type daemon struct {
	c       config.Config
	log     *slog.Logger
	store   *catalog.Store
	worker  *worker.Worker
	watcher *inbox.Watcher
	wake    chan struct{}
}

func (f *flags) daemon(cmd *cobra.Command, app App) (*daemon, error) {
	c, err := f.load(cmd, app)
	if err != nil {
		return nil, err
	}
	d := &daemon{c: c, log: f.logger(app), wake: make(chan struct{}, 1)}
	if d.store, err = openStore(c); err != nil {
		return nil, err
	}
	if d.worker, err = newWorker(app, c, d.store, d.log); err != nil {
		return nil, err
	}
	d.watcher = &inbox.Watcher{
		Dir: c.Inbox, Settle: c.Settle, Log: d.log,
		Ingest: func(path string) error {
			e, created, err := d.store.Ingest(path)
			if err != nil {
				return err //nolint:wrapcheck // logged by the watcher with the file name
			}
			if !created {
				d.log.Info("already seen", "file", path, "plan", e.ID, "status", e.Status)
				return nil
			}
			d.log.Info("queued", "file", path, "plan", e.ID)
			d.poke()
			return nil
		},
	}
	return d, nil
}

// poke wakes an idle worker. The channel holds one wake-up; more would only
// say the same thing.
func (d *daemon) poke() {
	select {
	case d.wake <- struct{}{}:
	default:
	}
}

// run runs the watcher, the workers and any extra services until ctx is done
// or one of them stops, which stops the rest.
func (d *daemon) run(ctx context.Context, extra ...func(context.Context) error) error {
	d.log.Info("watching", "inbox", d.c.Inbox, "output", d.c.Output, "model", d.c.Model,
		"workers", d.c.Workers, "maxUsd", d.c.Limits.MaxUSD)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	services := append([]func(context.Context) error{
		d.watcher.Run,
		func(ctx context.Context) error { return d.worker.Run(ctx, d.c.Workers, d.wake) },
	}, extra...)
	errs := make([]error, len(services))
	var wg sync.WaitGroup
	for i, run := range services {
		wg.Go(func() {
			errs[i] = run(ctx)
			cancel()
		})
	}
	wg.Wait()
	d.log.Info("stopped")
	return errors.Join(errs...)
}

func watchCmd(app App, f *flags) *cobra.Command {
	return &cobra.Command{
		Use:   "watch",
		Short: "Watch the inbox and trace each new image",
		Long: "Watches the inbox folder. Each new image is copied into the output folder, queued, " +
			"traced and published as a draft. Runs until interrupted.",
		Args: usageArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			d, err := f.daemon(cmd, app)
			if err != nil {
				return err
			}
			return d.run(cmd.Context())
		},
	}
}

func serveCmd(app App, f *flags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the microsite with uploads and review, and trace what arrives",
		Long: "Serves the microsite. Images dropped on the page (or into the inbox) are traced " +
			"and published as drafts, which can be approved or sent back from the page. " +
			"Runs until interrupted.\n\n" +
			"Only loopback host names are answered unless others are allowed, because every " +
			"upload spends money on the Claude API.",
		Args: usageArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			d, err := f.daemon(cmd, app)
			if err != nil {
				return err
			}
			s := d.c.Serve
			if _, err := os.Stat(filepath.Join(s.Site, "index.html")); err != nil {
				return fmt.Errorf(
					"site folder: %w (run from the repository root, or set serve.site)",
					err,
				)
			}
			ln, err := (&net.ListenConfig{}).Listen(cmd.Context(), "tcp", s.Addr)
			if err != nil {
				return fmt.Errorf("listen: %w", err)
			}
			hosts := s.Hosts
			h, _, _ := net.SplitHostPort(ln.Addr().String())
			if ip := net.ParseIP(h); ip != nil && ip.IsUnspecified() {
				if len(s.Hosts) == 0 {
					d.log.Warn(
						"listening on every interface, but answering only to localhost names; add others with --allow-host",
					)
				}
			} else {
				hosts = append(
					slices.Clone(hosts),
					strings.ToLower(h),
				) // a named interface answers to its own address
			}
			srv := &server.Server{
				Site:  s.Site,
				Store: d.store,
				Hosts: hosts,
				Wake:  d.poke,
				Log:   d.log,
				Info: server.Info{
					Version: buildinfo.Version(),
					Model:   d.c.Model,
					MaxUSD:  d.c.Limits.MaxUSD,
				},
			}
			hs := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
			d.log.Info("serving", "url", "http://"+ln.Addr().String()+"/", "site", s.Site)
			return d.run(cmd.Context(), func(ctx context.Context) error {
				served := make(chan error, 1)
				go func() { served <- hs.Serve(ln) }()
				select {
				case err := <-served:
					return fmt.Errorf("serve: %w", err)
				case <-ctx.Done():
				}
				// Uploads in flight get a moment to land; a trace in progress
				// is requeued by the worker, not waited for.
				stop, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
				defer cancel()
				if err := hs.Shutdown(stop); err != nil {
					return fmt.Errorf("shut down: %w", err)
				}
				return nil
			})
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&f.addr, "addr", "", "listen address (default "+config.Default().Serve.Addr+")")
	fl.StringVar(&f.site, "site", "", "the microsite's folder")
	fl.StringSliceVar(&f.hosts, "allow-host", nil, "extra host name to answer to (repeatable)")
	return cmd
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
			return f.review(cmd, app, args[0], catalog.Approve, "")
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
			return f.review(cmd, app, args[0], catalog.Reject, note)
		},
	}
	reject.Flags().StringVar(&note, "note", "", "what to fix")

	retry := &cobra.Command{
		Use:   "retry <id>",
		Short: "Re-queue a failed plan",
		Args:  usageArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return f.review(cmd, app, args[0], catalog.Retry, "")
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

func (f *flags) review(
	cmd *cobra.Command,
	app App,
	id string,
	a catalog.Action,
	note string,
) error {
	store, err := f.store(cmd, app)
	if err != nil {
		return err
	}
	was, e, err := store.Review(id, a, note)
	if err != nil {
		return err //nolint:wrapcheck // names the plan
	}
	_, _ = fmt.Fprintf(app.Stdout, "%s: %s → %s\n", id, was, e.Status)
	return nil
}
