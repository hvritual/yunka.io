package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/urfave/cli"
	"yunka.io/app/cmd/sourceaudit"
)

func sourceCommand() cli.Command {
	return cli.Command{
		Name: "source", Usage: "inventory all owned Go source and check an explicit module/build-profile import policy (read-only)",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: ".", Usage: "root containing owned modules, policy and every local workspace/replace target"},
			cli.StringFlag{Name: "policy", Usage: "required root-contained source-policy JSON file"},
			cli.StringFlag{Name: "format", Value: "text", Usage: "text, json or agent-json"},
			cli.DurationFlag{Name: "timeout", Value: 2 * time.Minute, Usage: "total analysis budget (maximum ten minutes)"},
		},
		Action: func(c *cli.Context) error {
			format := c.String("format")
			if format != "text" && format != "json" && format != "agent-json" {
				return fmt.Errorf("source audit: unsupported format %q", format)
			}
			if c.String("policy") == "" {
				return fmt.Errorf("source audit requires --policy")
			}
			budget := c.Duration("timeout")
			if budget <= 0 || budget > 10*time.Minute {
				return fmt.Errorf("source audit timeout must be positive and at most ten minutes")
			}
			ctx, cancel := context.WithTimeout(context.Background(), budget)
			defer cancel()
			r, err := sourceaudit.Check(ctx, c.String("root"), c.String("policy"))
			if err != nil {
				return err
			}
			if format == "text" {
				_, err = fmt.Fprintf(c.App.Writer, "source audit: %s; inventoried=%d Go files; checked=%d excluded=%d uncovered=%d; profiles=%d/%d\n", r.Status, r.Inventory.GoFiles, r.Analysis.CheckedGoFiles, r.Analysis.ExcludedGoFiles, r.Analysis.UncoveredGoFiles, r.Analysis.CompletedProfiles, r.Analysis.RequiredProfiles)
				if err != nil {
					return err
				}
				for _, f := range r.Findings {
					if _, err = fmt.Fprintf(c.App.Writer, "%s %s %s:%d [%s] %s\n", f.Rule, f.Class, f.File, f.Line, f.Profile, f.Message); err != nil {
						return err
					}
				}
			} else {
				var b []byte
				b, err = r.Marshal()
				if err != nil {
					return err
				}
				_, err = c.App.Writer.Write(b)
				if err != nil {
					return err
				}
			}
			return r.Error()
		},
	}
}
