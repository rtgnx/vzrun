package cmds

import (
	"context"
	"fmt"
	"os"

	cli "github.com/jawher/mow.cli"
	apiv1 "github.com/rtgnx/vzrun/internal/api/v1"
)

func RunCmd(ctx context.Context, client *apiv1.Client) cli.CmdInitializer {
	return vmCreateCmd(ctx, client, true)
}

func CreateCmd(ctx context.Context, client *apiv1.Client) cli.CmdInitializer {
	return vmCreateCmd(ctx, client, false)
}

func vmCreateCmd(ctx context.Context, client *apiv1.Client, start bool) cli.CmdInitializer {
	return func(c *cli.Cmd) {
		var (
			name string
			req  apiv1.CreateVMRequest
		)

		c.IntOptPtr(&req.CPUs, `c cpus`, 1, `cpu count`)
		c.IntOptPtr(&req.MemoryGB, `m memory`, 1, `memory (GB)`)
		c.StringOptPtr(&name, `n name`, `test`, `vm name`)
		c.StringArgPtr(&req.Image, `IMAGE`, ``, `oci image`)

		c.Action = func() {
			if err := client.Create(ctx, name, req); err != nil {
				exitErr(err)
			}
			if start {
				if err := client.Start(ctx, name); err != nil {
					exitErr(err)
				}
			}
		}
	}
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
