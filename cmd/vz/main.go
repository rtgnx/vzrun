package main

import (
	"context"
	"os"
	"os/signal"
	"path"
	"runtime"
	"syscall"

	cli "github.com/jawher/mow.cli"
	"github.com/rtgnx/vzrun/internal/vz"
	apiv1 "github.com/rtgnx/vzrun/pkg/api/v1"
)

func init() {
	runtime.LockOSThread()
}

func main() {
	ctx, cancel := signal.NotifyContext(
		context.Background(), os.Interrupt, os.Kill, syscall.SIGTERM,
	)

	defer cancel()

	app := cli.App(`vz`, `run lightweight Linux VMs on macOS`)

	var (
		socketPath string
	)

	app.StringOptPtr(&socketPath, `s socket`, path.Join(os.Getenv(`HOME`), `.vzrun/vzd.sock`), `daemon socket path`)

	client := apiv1.NewClient(socketPath)

	app.Command(`create`, `create a VM from an OCI image`, vz.CreateCmd(ctx, client))
	app.Command(`run`, `create and start a VM`, vz.RunCmd(ctx, client))
	app.Command(`ps`, `list VMs and their states`, vz.PSCmd(ctx, client))
	app.Command(`start`, `start an existing VM`, vz.StartCmd(ctx, client))
	app.Command(`stop`, `stop a running VM`, vz.StopCmd(ctx, client))
	app.Command(`restart`, `restart a VM`, vz.RestartCmd(ctx, client))
	app.Command(`attach`, `attach to a VM console`, vz.AttachCmd(ctx, client))
	app.Command(`logs`, `stream VM console logs`, vz.LogsCmd(ctx, client))
	app.Command(`delete`, `delete a stopped VM`, vz.DeleteCmd(ctx, client))

	app.Run(os.Args)
}
