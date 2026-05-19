package cmds

import (
	"context"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	cli "github.com/jawher/mow.cli"
	apiv1 "github.com/rtgnx/vzrun/internal/api/v1"
)

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
