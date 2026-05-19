package vz

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"text/tabwriter"

	cli "github.com/jawher/mow.cli"
	apiv1 "github.com/rtgnx/vzrun/pkg/api/v1"
)

const (
	nameHelp        = "VM name"
	imageHelp       = "OCI image reference"
	interactiveHelp = "stream console output after the VM starts"
	ttyHelp         = "attach stdin to the VM console"
)

func StartCmd(ctx context.Context, client *apiv1.Client) cli.CmdInitializer {
	return func(c *cli.Cmd) {
		var (
			name        string
			interactive bool
			tty         bool
		)

		c.BoolOptPtr(&interactive, "i interactive", false, interactiveHelp)
		c.BoolOptPtr(&tty, "t tty", false, ttyHelp)
		c.StringArgPtr(&name, "NAME", "test", nameHelp)

		c.Action = func() {
			if err := startVM(ctx, client, name, interactive, tty); err != nil {
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

		c.StringArgPtr(&name, "NAME", "test", nameHelp)

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

func startVM(ctx context.Context, client *apiv1.Client, name string, interactive, tty bool) error {
	if tty && !interactive {
		return errors.New("-t requires -i")
	}
	if err := client.Start(ctx, name); err != nil {
		return err
	}
	if !interactive {
		return nil
	}
	if tty {
		return attach(ctx, client, name)
	}
	return streamLogs(ctx, client, name)
}

func RestartCmd(ctx context.Context, client *apiv1.Client) cli.CmdInitializer {
	return ctlCmd(ctx, client, client.Restart)
}

func DeleteCmd(ctx context.Context, client *apiv1.Client) cli.CmdInitializer {
	return func(c *cli.Cmd) {
		var name string

		c.StringArgPtr(&name, "NAME", "test", nameHelp)

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

		c.StringArgPtr(&name, "NAME", "test", nameHelp)

		c.Action = func() {
			if err := ctl(ctx, name); err != nil {
				exitErr(err)
			}
		}
	}
}

func LogsCmd(ctx context.Context, client *apiv1.Client) cli.CmdInitializer {
	return func(c *cli.Cmd) {
		var name string

		c.StringArgPtr(&name, "NAME", "test", nameHelp)

		c.Action = func() {
			if err := streamLogs(ctx, client, name); err != nil {
				exitErr(err)
			}
		}
	}
}

func PSCmd(ctx context.Context, client *apiv1.Client) cli.CmdInitializer {
	return func(c *cli.Cmd) {
		c.Action = func() {
			res, err := client.List(ctx)
			if err != nil {
				exitErr(err)
			}

			names := make([]string, 0, len(res.List))
			for name := range res.List {
				names = append(names, name)
			}
			sort.Strings(names)

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tSTATE")
			for _, name := range names {
				fmt.Fprintf(w, "%s\t%s\n", name, res.List[name])
			}
			w.Flush()
		}
	}
}

func RunCmd(ctx context.Context, client *apiv1.Client) cli.CmdInitializer {
	return vmCreateCmd(ctx, client, true)
}

func CreateCmd(ctx context.Context, client *apiv1.Client) cli.CmdInitializer {
	return vmCreateCmd(ctx, client, false)
}

func vmCreateCmd(ctx context.Context, client *apiv1.Client, start bool) cli.CmdInitializer {
	return func(c *cli.Cmd) {
		var (
			name        string
			req         apiv1.CreateVMRequest
			interactive bool
			tty         bool
		)

		c.IntOptPtr(&req.CPUs, `c cpus`, 1, `number of vCPUs`)
		c.IntOptPtr(&req.MemoryGB, `m memory`, 1, `memory size in GB`)
		c.StringOptPtr(&name, `n name`, `test`, nameHelp)
		if start {
			c.BoolOptPtr(&interactive, "i interactive", false, interactiveHelp)
			c.BoolOptPtr(&tty, "t tty", false, ttyHelp)
		}
		c.StringArgPtr(&req.Image, `IMAGE`, ``, imageHelp)

		c.Action = func() {
			if err := client.Create(ctx, name, req); err != nil {
				exitErr(err)
			}
			if start {
				if err := startVM(ctx, client, name, interactive, tty); err != nil {
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
