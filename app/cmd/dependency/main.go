package dependency

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/urfave/cli"
	"github.com/hvritual/yunka.io/pkg/dependencypolicy"
)

const AppName = "dependency"

func Command() cli.Command {
	return cli.Command{
		Name:  AppName,
		Usage: "inspect and enforce the repository dependency convergence policy",
		Subcommands: []cli.Command{
			{
				Name:  "check",
				Usage: "fail when the workspace dependency graph or legacy imports violate policy",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "repo-root", Value: ".", Usage: "repository root"},
					cli.StringFlag{Name: "policy", Value: "tools/dependency-policy.json", Usage: "dependency policy JSON"},
					cli.StringFlag{Name: "go", Value: "go", EnvVar: "GO", Usage: "Go command used to inspect the module graph"},
				},
				Action: check,
			},
		},
	}
}

func check(c *cli.Context) error {
	root, err := filepath.Abs(c.String("repo-root"))
	if err != nil {
		return err
	}
	policyPath := strings.TrimSpace(c.String("policy"))
	if !filepath.IsAbs(policyPath) {
		policyPath = filepath.Join(root, policyPath)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	diagnostics, err := dependencypolicy.Check(ctx, root, policyPath, c.String("go"))
	if err != nil {
		return err
	}
	for _, diagnostic := range diagnostics {
		fmt.Printf("DEPENDENCY %s: %s\n", diagnostic.Path, diagnostic.Message)
	}
	if len(diagnostics) != 0 {
		return errors.New("dependency policy check failed")
	}
	fmt.Println("dependency policy ok")
	return nil
}
