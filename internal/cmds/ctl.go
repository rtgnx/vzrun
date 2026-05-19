package cmds

import (
	"context"
	"errors"
	"io"
	"os"

	cli "github.com/jawher/mow.cli"
	apiv1 "github.com/rtgnx/vzrun/internal/api/v1"
)

func StartCmd(ctx context.Context, client *apiv1.Client) cli.CmdInitializer {
	return func(c *cli.Cmd) {
		var (
			name        string
			interactive bool
			tty         bool
		)

		c.BoolOptPtr(&interactive, "i interactive", false, "stream console output after starting")
		c.BoolOptPtr(&tty, "t tty", false, "attach stdin as well as console output")
		c.StringArgPtr(&name, "NAME", "test", "vm name")

		c.Action = func() {
			if tty && !interactive {
				exitErr(errors.New("-t requires -i"))
			}
			if err := client.Start(ctx, name); err != nil {
				exitErr(err)
			}
			if !interactive {
				return
			}
			if tty {
				if err := attach(ctx, client, name); err != nil {
					exitErr(err)
				}
				return
			}
			if err := streamLogs(ctx, client, name); err != nil {
				exitErr(err)
			}
		}
	}
}

func StopCmd(ctx context.Context, client *apiv1.Client) cli.CmdInitializer {
	return ctlCmd(ctx, client, client.Stop)
}

func AttachCmd(ctx context.Context, client *apiv1.Client) cli.CmdInitializer {
	return func(c *cli.Cmd) {
		var name string

		c.StringArgPtr(&name, "NAME", "test", "vm name")

		c.Action = func() {
			if err := attach(ctx, client, name); err != nil {
				exitErr(err)
			}
		}
	}
}

func streamLogs(ctx context.Context, client *apiv1.Client, name string) error {
	r, err := client.Logs(ctx, name)
	if err != nil {
		return err
	}
	defer r.Close()

	_, err = io.Copy(os.Stdout, r)
	if err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}

func attach(ctx context.Context, client *apiv1.Client, name string) error {
	r, err := client.Attach(ctx, name, os.Stdin)
	if err != nil {
		return err
	}
	defer r.Close()

	_, err = io.Copy(os.Stdout, r)
	if err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}

func RestartCmd(ctx context.Context, client *apiv1.Client) cli.CmdInitializer {
	return ctlCmd(ctx, client, client.Restart)
}

func DeleteCmd(ctx context.Context, client *apiv1.Client) cli.CmdInitializer {
	return func(c *cli.Cmd) {
		var name string

		c.StringArgPtr(&name, "NAME", "test", "vm name")

		c.Action = func() {
			if err := client.Delete(ctx, name); err != nil {
				exitErr(err)
			}
		}
	}
}

func ctlCmd(ctx context.Context, _ *apiv1.Client, ctl func(context.Context, string) error) cli.CmdInitializer {
	return func(c *cli.Cmd) {
		var name string

		c.StringArgPtr(&name, "NAME", "test", "vm name")

		c.Action = func() {
			if err := ctl(ctx, name); err != nil {
				exitErr(err)
			}
		}
	}
}
