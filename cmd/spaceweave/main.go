// Command spaceweave traces floor-plan images into the isometric microsite.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/weaveplatform/spaceweave/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := cli.Execute(ctx, cli.App{Stdout: os.Stdout, Stderr: os.Stderr, Getenv: os.LookupEnv}, os.Args[1:])
	stop()
	os.Exit(code)
}
