package audit

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	auditcore "github.com/hvritual/yunka.io/pkg/audit"
	"github.com/hvritual/yunka.io/pkg/contract"
	"github.com/urfave/cli"
	"yunka.io/app/cmd/projectflow"
)

const AppName = "audit"

func Command() cli.Command {
	return cli.Command{
		Name:  AppName,
		Usage: "inspect deterministic framework-conformance evidence without mutating the project",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
			cli.StringFlag{Name: "format", Value: "text", Usage: "output format: text, json, or agent-json"},
		},
		Action: func(c *cli.Context) error {
			report, err := Build(c.String("root"))
			if err != nil {
				return err
			}
			output, err := Render(report, c.String("format"))
			if err != nil {
				return err
			}
			fmt.Print(output)
			return nil
		},
	}
}

func Build(root string) (auditcore.Report, error) {
	descriptor, err := projectflow.DescribeProject(projectflow.Options{Root: root})
	if err != nil {
		return auditcore.Report{}, fmt.Errorf("audit: resolve project: %w", err)
	}
	source, err := auditcore.CollectGoSource(descriptor.Root, descriptor.GeneratedGoRoot)
	if err != nil {
		return auditcore.Report{}, err
	}
	manifestPath := filepath.Join(projectflow.ResolveDescriptorPath(descriptor, descriptor.ContractGenerated), contract.ManifestFilename)
	manifest, err := contract.LoadManifest(manifestPath)
	if err != nil {
		return auditcore.Report{}, fmt.Errorf("audit: load canonical manifest %s: %w; run `yunka generate` first", filepath.ToSlash(manifestPath), err)
	}

	report := auditcore.NewReport(auditcore.ProjectIdentity{GoModule: descriptor.GoModule, Profiled: descriptor.Profiled})
	report.Source = source
	report.Findings = auditcore.EvaluateSource(source, auditcore.RuleOptions{
		GoModule:        descriptor.GoModule,
		GeneratedGoRoot: descriptor.GeneratedGoRoot,
		DeclaredDomains: declaredDomains(manifest),
	})
	auditcore.Normalize(&report)
	if err := auditcore.Validate(report); err != nil {
		return auditcore.Report{}, err
	}
	return report, nil
}

func Render(report auditcore.Report, format string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	switch format {
	case "json", "agent-json":
		contents, err := auditcore.Marshal(report)
		if err != nil {
			return "", err
		}
		return string(contents), nil
	case "", "text":
		var builder strings.Builder
		module := strings.TrimSpace(report.Project.GoModule)
		if module == "" {
			module = "<unknown>"
		}
		fmt.Fprintf(&builder, "PROJECT module=%s profiled=%t\n", module, report.Project.Profiled)
		fmt.Fprintf(&builder, "SOURCE  root=%s files=%d\n", report.Source.SourceRoot, len(report.Source.Files))
		fmt.Fprintf(&builder, "FINDINGS %d\n", len(report.Findings))
		for _, finding := range report.Findings {
			fmt.Fprintf(&builder, "  %s %s %s — %s\n", finding.Class, finding.Rule, finding.Subject, finding.Summary)
		}
		return builder.String(), nil
	default:
		return "", fmt.Errorf("audit: unsupported format %q; use text, json, or agent-json", format)
	}
}

func declaredDomains(manifest contract.Manifest) []string {
	manifest.Normalize()
	seen := map[string]struct{}{}
	for _, file := range manifest.Files {
		if file.Domain == nil {
			continue
		}
		if name := strings.TrimSpace(file.Domain.Name); name != "" {
			seen[name] = struct{}{}
		}
	}
	for _, service := range manifest.Services {
		if name := strings.TrimSpace(service.Domain); name != "" {
			seen[name] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for name := range seen {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}
