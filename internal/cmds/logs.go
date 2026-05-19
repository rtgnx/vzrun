package cmds

import (
	"context"

	cli "github.com/jawher/mow.cli"
	apiv1 "github.com/rtgnx/vzrun/internal/api/v1"
)

func LogsCmd(ctx context.Context, client *apiv1.Client) cli.CmdInitializer {
	return func(c *cli.Cmd) {
		var name string

		c.StringArgPtr(&name, "NAME", "test", "vm name")

		c.Action = func() {
			if err := streamLogs(ctx, client, name); err != nil {
				exitErr(err)
			}
		}
	}
}
