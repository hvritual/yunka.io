package audit

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hvritual/yunka.io/pkg/contract"
	"github.com/urfave/cli"
	"yunka.io/app/cmd/auditcore"
	"yunka.io/app/cmd/projectflow"
)

const AppName = "audit"

func Command() cli.Command {
	return cli.Command{
		Name:  AppName,
		Usage: "inspect deterministic framework-conformance evidence without mutating the project",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
			cli.StringFlag{Name: "base", Usage: "optional Git ref used to classify proven findings as existing, new, or fixed debt"},
			cli.StringFlag{Name: "format", Value: "text", Usage: "output format: text, json, or agent-json"},
		},
		Action: func(c *cli.Context) error {
			var report auditcore.Report
			var err error
			if strings.TrimSpace(c.String("base")) == "" {
				report, err = Build(c.String("root"))
			} else {
				report, err = BuildWithBase(c.String("root"), c.String("base"))
			}
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
	report, _, err := buildCurrent(root)
	return report, err
}

func BuildWithBase(root, baseRef string) (auditcore.Report, error) {
	current, descriptor, err := buildCurrent(root)
	if err != nil {
		return auditcore.Report{}, err
	}
	baseRef = strings.TrimSpace(baseRef)
	baseSHA, err := auditcore.ResolveGitCommit(descriptor.Root, baseRef)
	if err != nil {
		return auditcore.Report{}, err
	}
	if descriptor.GoModule != "" {
		goMod, err := auditcore.ReadGitFileAtCommit(descriptor.Root, baseSHA, "go.mod")
		if err != nil {
			return auditcore.Report{}, fmt.Errorf("audit debt: baseline project identity: %w", err)
		}
		baseModule := goModuleIdentity(goMod)
		if baseModule == "" {
			return auditcore.Report{}, fmt.Errorf("audit debt: baseline go.mod has no module identity")
		}
		if baseModule != descriptor.GoModule {
			return auditcore.Report{}, fmt.Errorf("audit debt: baseline module %q differs from current module %q; choose a baseline after the module-identity migration", baseModule, descriptor.GoModule)
		}
	}

	baseSource, err := auditcore.CollectGoSourceAtCommit(descriptor.Root, descriptor.GeneratedGoRoot, baseSHA)
	if err != nil {
		return auditcore.Report{}, err
	}
	manifestRelative, err := projectRelativePath(descriptor.Root, projectflow.ResolveDescriptorPath(descriptor, filepath.Join(descriptor.ContractGenerated, contract.ManifestFilename)))
	if err != nil {
		return auditcore.Report{}, err
	}
	manifestBytes, err := auditcore.ReadGitFileAtCommit(descriptor.Root, baseSHA, manifestRelative)
	if err != nil {
		return auditcore.Report{}, fmt.Errorf("audit debt: baseline canonical manifest: %w", err)
	}
	var baseManifest contract.Manifest
	if err := json.Unmarshal(manifestBytes, &baseManifest); err != nil {
		return auditcore.Report{}, fmt.Errorf("audit debt: decode baseline canonical manifest %s: %w", manifestRelative, err)
	}
	baseManifest.Normalize()
	baseFindings := auditcore.EvaluateSource(baseSource, auditcore.RuleOptions{
		GoModule:        descriptor.GoModule,
		GeneratedGoRoot: descriptor.GeneratedGoRoot,
		DeclaredDomains: declaredDomains(baseManifest),
	})
	debt := auditcore.CompareProvenFindings(baseFindings, current.Findings)
	debt.BaseRef = baseRef
	debt.BaseSHA = baseSHA
	current.Debt = &debt
	auditcore.Normalize(&current)
	if err := auditcore.Validate(current); err != nil {
		return auditcore.Report{}, err
	}
	return current, nil
}

func buildCurrent(root string) (auditcore.Report, projectflow.ProjectDescriptor, error) {
	descriptor, err := projectflow.DescribeProject(projectflow.Options{Root: root})
	if err != nil {
		return auditcore.Report{}, projectflow.ProjectDescriptor{}, fmt.Errorf("audit: resolve project: %w", err)
	}
	source, err := auditcore.CollectGoSource(descriptor.Root, descriptor.GeneratedGoRoot)
	if err != nil {
		return auditcore.Report{}, projectflow.ProjectDescriptor{}, err
	}
	manifestPath := filepath.Join(projectflow.ResolveDescriptorPath(descriptor, descriptor.ContractGenerated), contract.ManifestFilename)
	manifest, err := contract.LoadManifest(manifestPath)
	if err != nil {
		return auditcore.Report{}, projectflow.ProjectDescriptor{}, fmt.Errorf("audit: load canonical manifest %s: %w; run `yunka generate` first", filepath.ToSlash(manifestPath), err)
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
		return auditcore.Report{}, projectflow.ProjectDescriptor{}, err
	}
	return report, descriptor, nil
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
		if report.Debt != nil {
			fmt.Fprintf(&builder, "DEBT base=%s sha=%s existing=%d new=%d fixed=%d\n", report.Debt.BaseRef, report.Debt.BaseSHA, len(report.Debt.Existing), len(report.Debt.New), len(report.Debt.Fixed))
			for _, finding := range report.Debt.New {
				fmt.Fprintf(&builder, "  NEW   %s %s — %s\n", finding.Rule, finding.Subject, finding.Summary)
			}
			for _, finding := range report.Debt.Fixed {
				fmt.Fprintf(&builder, "  FIXED %s %s — %s\n", finding.Rule, finding.Subject, finding.Summary)
			}
		}
		return builder.String(), nil
	default:
		return "", fmt.Errorf("audit: unsupported format %q; use text, json, or agent-json", format)
	}
}

func projectRelativePath(root, target string) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("audit debt: project root: %w", err)
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("audit debt: canonical path: %w", err)
	}
	relative, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("audit debt: canonical path %s is outside project root", target)
	}
	return filepath.ToSlash(relative), nil
}

func goModuleIdentity(contents []byte) string {
	for _, line := range strings.Split(string(contents), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
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
