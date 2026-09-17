package audit

import (
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
		Name:        AppName,
		Subcommands: []cli.Command{typesCommand(), sourceCommand()},
		Usage:       "inspect deterministic framework-conformance evidence without mutating the project",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
			cli.StringFlag{Name: "base", Usage: "optional Git ref used to classify proven findings as existing, new, or fixed debt"},
			cli.StringFlag{Name: "protoc", EnvVar: "PROTOC", Usage: "protoc binary used for base/current Operation Growth evidence"},
			cli.StringSliceFlag{Name: "proto-path", Usage: "additional protobuf include path used for base/current Operation Growth evidence; may be repeated"},
			cli.StringFlag{Name: "format", Value: "text", Usage: "output format: text, json, or agent-json"},
		},
		Action: func(c *cli.Context) error {
			var report auditcore.Report
			var err error
			if strings.TrimSpace(c.String("base")) == "" {
				report, err = Build(c.String("root"))
			} else {
				report, err = BuildWithBaseOptions(projectflow.Options{Root: c.String("root"), Protoc: c.String("protoc"), ProtoPaths: c.StringSlice("proto-path")}, c.String("base"))
			}
			if err != nil {
				return err
			}
			output, err := Render(report, c.String("format"))
			if err != nil {
				return err
			}
			fmt.Print(output)
			if blocking := auditcore.BlockingNewFindings(report); len(blocking) > 0 {
				return fmt.Errorf("audit: %d new blocking engineering-quality finding(s); inspect the rendered debt delta", len(blocking))
			}
			return nil
		},
	}
}

func Build(root string) (auditcore.Report, error) {
	report, _, err := buildCurrent(root)
	return report, err
}

func BuildWithBase(root, baseRef string) (auditcore.Report, error) {
	return BuildWithBaseOptions(projectflow.Options{Root: root}, baseRef)
}

func BuildWithBaseOptions(options projectflow.Options, baseRef string) (auditcore.Report, error) {
	current, descriptor, err := buildCurrent(options.Root)
	if err != nil {
		return auditcore.Report{}, err
	}
	baseRef = strings.TrimSpace(baseRef)
	baseSHA, err := auditcore.ResolveGitCommit(descriptor.Root, baseRef)
	if err != nil {
		return auditcore.Report{}, err
	}
	baselineRoot, cleanup, err := auditcore.MaterializeGitCommit(descriptor.Root, baseSHA)
	if err != nil {
		return auditcore.Report{}, err
	}
	defer cleanup()
	baseline, baselineDescriptor, err := buildCurrent(baselineRoot)
	if err != nil {
		return auditcore.Report{}, fmt.Errorf("audit debt: evaluate immutable baseline %s: %w", baseSHA, err)
	}
	if descriptor.GoModule != baselineDescriptor.GoModule {
		return auditcore.Report{}, fmt.Errorf("audit debt: baseline module %q differs from current module %q; choose a baseline after the module-identity migration", baselineDescriptor.GoModule, descriptor.GoModule)
	}
	growth, err := boundaryGrowthFindings(projectflow.Options{Root: descriptor.Root, Protoc: options.Protoc, ProtoPaths: append([]string(nil), options.ProtoPaths...)}, baselineRoot, baseSHA)
	if err != nil {
		return auditcore.Report{}, err
	}
	current.Findings = append(current.Findings, growth...)
	debt := auditcore.CompareProvenFindings(baseline.Findings, current.Findings)
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
	qualityPolicy, policyEvidence, err := auditcore.LoadQualityPolicy(descriptor.Root)
	if err != nil {
		return auditcore.Report{}, projectflow.ProjectDescriptor{}, err
	}

	report := auditcore.NewReport(auditcore.ProjectIdentity{GoModule: descriptor.GoModule, Profiled: descriptor.Profiled})
	report.QualityPolicy = policyEvidence
	report.Source = source
	report.Findings = auditcore.EvaluateSource(source, auditcore.RuleOptions{
		GoModule:        descriptor.GoModule,
		GeneratedGoRoot: descriptor.GeneratedGoRoot,
		DeclaredDomains: declaredDomains(manifest),
		Limits:          qualityPolicy.Limits,
	})
	generated, err := generatedArtifactFindings(descriptor.Root, descriptor.GeneratedGoRoot)
	if err != nil {
		return auditcore.Report{}, projectflow.ProjectDescriptor{}, fmt.Errorf("audit: inspect generated ownership: %w", err)
	}
	report.Findings = append(report.Findings, generated...)
	auditcore.ApplyBlockingPolicy(report.Findings, qualityPolicy)
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
		fmt.Fprintf(&builder, "POLICY  path=%s present=%t blocking=%d\n", report.QualityPolicy.Path, report.QualityPolicy.Present, len(report.QualityPolicy.BlockingRules))
		fmt.Fprintf(&builder, "SOURCE  root=%s files=%d\n", report.Source.SourceRoot, len(report.Source.Files))
		fmt.Fprintf(&builder, "FINDINGS %d\n", len(report.Findings))
		for _, finding := range report.Findings {
			blocking := ""
			if finding.Blocking {
				blocking = " BLOCKING"
			}
			fmt.Fprintf(&builder, "  %s%s %s %s — %s\n", finding.Class, blocking, finding.Rule, finding.Subject, finding.Summary)
		}
		if report.Debt != nil {
			fmt.Fprintf(&builder, "DEBT base=%s sha=%s existing=%d new=%d fixed=%d blocking_new=%d\n", report.Debt.BaseRef, report.Debt.BaseSHA, len(report.Debt.Existing), len(report.Debt.New), len(report.Debt.Fixed), len(auditcore.BlockingNewFindings(report)))
			for _, finding := range report.Debt.New {
				blocking := ""
				if finding.Blocking {
					blocking = " BLOCKING"
				}
				fmt.Fprintf(&builder, "  NEW%s %s %s — %s\n", blocking, finding.Rule, finding.Subject, finding.Summary)
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
