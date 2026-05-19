//go:build darwin

package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"path"
	"syscall"

	cli "github.com/jawher/mow.cli"
	"github.com/rtgnx/vzrun/internal/api"
	"github.com/rtgnx/vzrun/internal/vzd"
)

func main() {
	app := cli.App("vzd", "vzrun daemon")
	var (
		socketPath string
	)

	app.StringOptPtr(&socketPath, `s socket`, path.Join(os.Getenv(`HOME`), `.vzrun/vzd.sock`), `socket path`)

	app.Action = func() {
		ctx, cancel := signal.NotifyContext(
			context.Background(), os.Interrupt, os.Kill, syscall.SIGTERM,
		)
		defer cancel()

		daemon, err := vzd.New(path.Join(os.Getenv(`HOME`), `.vzrun`))
		if err != nil {
			log.Fatal(err)
		}
		if err := api.Start(ctx, socketPath, daemon); err != nil && !errors.Is(err, context.Canceled) {
			log.Fatal(err)
		}
	}

	app.Run(os.Args)
}
