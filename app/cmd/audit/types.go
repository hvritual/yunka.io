package audit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hvritual/yunka.io/pkg/applicationboundary"
	"github.com/urfave/cli"
)

func typesCommand() cli.Command {
	return cli.Command{Name: "types", Usage: "check explicit factory/capability type boundaries (one module, read-only)", Flags: []cli.Flag{
		cli.StringFlag{Name: "root", Value: ".", Usage: "Go module root; dependency cache must be prepared"},
		cli.StringFlag{Name: "policy", Usage: "required explicit boundary policy JSON, relative to root"},
		cli.StringFlag{Name: "tags", Usage: "explicit active Go build tags"},
		cli.StringFlag{Name: "format", Value: "text", Usage: "text, json, or agent-json"},
		cli.DurationFlag{Name: "timeout", Value: 2 * time.Minute, Usage: "bounded package loading and analysis budget"},
	}, Action: func(c *cli.Context) error {
		if c.String("policy") == "" {
			return fmt.Errorf("audit types: --policy is required")
		}
		if c.Duration("timeout") <= 0 || c.Duration("timeout") > 10*time.Minute {
			return fmt.Errorf("audit types: timeout must be positive and at most 10m")
		}
		format := strings.ToLower(c.String("format"))
		if format != "text" && format != "json" && format != "agent-json" {
			return fmt.Errorf("audit types: unsupported format %q", format)
		}
		path := c.String("policy")
		if !filepath.IsAbs(path) {
			path = filepath.Join(c.String("root"), path)
		}
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("audit types: read policy: %w", err)
		}
		policy, err := applicationboundary.ReadPolicy(f)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		ctx, cancel := context.WithTimeout(context.Background(), c.Duration("timeout"))
		defer cancel()
		report := applicationboundary.Check(ctx, c.String("root"), policy, applicationboundary.Options{Tags: c.String("tags")})
		output, err := RenderTypes(report, format)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprint(c.App.Writer, output); err != nil {
			return err
		}
		return report.Error()
	}}
}
func RenderTypes(report applicationboundary.Report, format string) (string, error) {
	switch format {
	case "json", "agent-json":
		b, e := report.Marshal()
		return string(b), e
	case "", "text":
		var b strings.Builder
		fmt.Fprintf(&b, "TYPED AUDIT %s factories=%d references=%d packages=%d\n", report.Status, report.CheckedFactories, report.CheckedReferences, len(report.Packages))
		for _, f := range report.Findings {
			fmt.Fprintf(&b, "%s %s %s:%d:%d %s — %s\n", f.Rule, f.Class, f.File, f.Line, f.Column, f.Subject, f.Message)
		}
		return b.String(), nil
	default:
		return "", fmt.Errorf("audit types: unsupported format %q", format)
	}
}
