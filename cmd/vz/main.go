package main

import (
	"context"
	"os"
	"os/signal"
	"path"
	"runtime"
	"syscall"

	cli "github.com/jawher/mow.cli"
	apiv1 "github.com/rtgnx/vzrun/internal/api/v1"
	"github.com/rtgnx/vzrun/internal/cmds"
)

func init() {
	runtime.LockOSThread()
}

func main() {
	ctx, cancel := signal.NotifyContext(
		context.Background(), os.Interrupt, os.Kill, syscall.SIGTERM,
	)

	defer cancel()

	app := cli.App(`vz`, `apple virtualization vms`)

	var (
		socketPath string
	)

	app.StringOptPtr(&socketPath, `s socket`, path.Join(os.Getenv(`HOME`), `.vzrun/vzd.sock`), `socket path`)

	client := apiv1.NewClient(socketPath)

	app.Command(`create`, `create vm`, cmds.CreateCmd(ctx, client))
	app.Command(`run`, `run vm`, cmds.RunCmd(ctx, client))
	app.Command(`ps`, `list vms`, cmds.PSCmd(ctx, client))
	app.Command(`start`, `start vm`, cmds.StartCmd(ctx, client))
	app.Command(`stop`, `stop vm`, cmds.StopCmd(ctx, client))
	app.Command(`restart`, `restart vm`, cmds.RestartCmd(ctx, client))
	app.Command(`attach`, `attach to vm console`, cmds.AttachCmd(ctx, client))
	app.Command(`logs`, `stream vm logs`, cmds.LogsCmd(ctx, client))
	app.Command(`delete`, `delete vm`, cmds.DeleteCmd(ctx, client))

	app.Run(os.Args)
}
