package ownership

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/urfave/cli"
	"yunka.io/app/cmd/dxoutput"
	"yunka.io/app/cmd/projectflow"
)

const (
	AppName       = "ownership"
	SchemaVersion = 1

	MutationEditable      = "editable"
	MutationGeneratedOnly = "generated-only"
	MutationUnclassified  = "unclassified"
)

type Decision struct {
	Path         string `json:"path"`
	Owner        string `json:"owner"`
	Mutation     string `json:"mutation"`
	SafeAutoEdit bool   `json:"safeAutoEdit"`
	Reason       string `json:"reason"`
}

type Report struct {
	SchemaVersion int        `json:"schemaVersion"`
	ProjectRoot   string     `json:"projectRoot"`
	Decisions     []Decision `json:"decisions"`
}

type classifier struct {
	inputs            projectflow.OwnershipInputs
	sourceFiles       map[string]struct{}
	protobufGenerated map[string]struct{}
	domainGenerated   map[string]struct{}
}

func Command() cli.Command {
	return cli.Command{
		Name:  AppName,
		Usage: "classify proposed file mutations from canonical Yunka ownership facts",
		Subcommands: []cli.Command{
			ownershipCommand("inspect", false),
			ownershipCommand("check", true),
		},
	}
}

func ownershipCommand(name string, enforce bool) cli.Command {
	usage := "inspect ownership for one or more proposed paths"
	if enforce {
		usage = "fail unless every proposed path is safe for automatic editing"
	}
	return cli.Command{
		Name:  name,
		Usage: usage,
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
			cli.StringSliceFlag{Name: "path", Usage: "project path to classify; may be repeated"},
			cli.StringFlag{Name: "format", Value: dxoutput.FormatText, Usage: "output format: text or json"},
		},
		Action: func(c *cli.Context) error {
			paths := c.StringSlice("path")
			if len(paths) == 0 {
				return cli.NewExitError("ownership: at least one --path is required", 2)
			}
			report, err := Build(c.String("root"), paths)
			if err != nil {
				return err
			}
			output, err := render(report, c.String("format"))
			if err != nil {
				return err
			}
			fmt.Print(output)
			if enforce {
				for _, decision := range report.Decisions {
					if !decision.SafeAutoEdit {
						return cli.NewExitError("", 1)
					}
				}
			}
			return nil
		},
	}
}

func Build(root string, paths []string) (Report, error) {
	inputs, err := projectflow.DescribeOwnershipInputs(projectflow.Options{Root: root})
	if err != nil {
		return Report{}, err
	}
	current := newClassifier(inputs)
	decisions := make([]Decision, 0, len(paths))
	for _, path := range paths {
		decisions = append(decisions, current.classify(path))
	}
	sort.Slice(decisions, func(i, j int) bool { return decisions[i].Path < decisions[j].Path })
	return Report{SchemaVersion: SchemaVersion, ProjectRoot: inputs.Project.Root, Decisions: decisions}, nil
}

func newClassifier(inputs projectflow.OwnershipInputs) classifier {
	return classifier{
		inputs:            inputs,
		sourceFiles:       pathSet(inputs.ContractSourceFiles),
		protobufGenerated: pathSet(inputs.ProtobufGoGeneratedFiles),
		domainGenerated:   pathSet(inputs.DomainGeneratedFiles),
	}
}

func (current classifier) classify(path string) Decision {
	relative, ok := normalizePath(current.inputs.Project.Root, path)
	if !ok {
		return decision(filepath.ToSlash(path), "unclassified", MutationUnclassified, false, "path is outside the canonical project root")
	}

	project := current.inputs.Project
	if within(relative, project.ContractGenerated) {
		return decision(relative, "yunka-generator", MutationGeneratedOnly, false, "path is inside the canonical generated contract artifact root")
	}
	if _, generated := current.protobufGenerated[relative]; generated {
		return decision(relative, "protobuf-go-generator", MutationGeneratedOnly, false, "path is owned by .yunka/protobuf-go.json")
	}
	if _, generated := current.domainGenerated[relative]; generated {
		return decision(relative, "yunka-domain-generator", MutationGeneratedOnly, false, "path is derived from the canonical Domain manifest/PO render")
	}
	if relative == clean(project.ProtobufGoManifest) {
		return decision(relative, "protobuf-go-generator", MutationGeneratedOnly, false, "protobuf output ownership manifest is maintained by Yunka generation")
	}
	if generatedByMarker(project.Root, relative) {
		return decision(relative, "yunka-generator", MutationGeneratedOnly, false, "existing file carries a recognized generated-code marker")
	}
	if within(relative, project.GeneratedGoRoot) && reservedGeneratedPath(relative) {
		return decision(relative, "yunka-generator", MutationGeneratedOnly, false, "path uses a generator-reserved filename")
	}

	if _, source := current.sourceFiles[relative]; source {
		return decision(relative, "developer-contract", MutationEditable, true, "path is a canonical protobuf contract source")
	}
	if project.ContractSourceKind == "proto-root" && within(relative, project.ContractSource) && strings.EqualFold(filepath.Ext(relative), ".proto") {
		return decision(relative, "developer-contract", MutationEditable, true, "new or existing protobuf file is inside the canonical proto root")
	}
	if project.ContractSourceKind == "inventory" && relative == clean(project.ContractSource) {
		return decision(relative, "developer-config", MutationEditable, true, "path is the canonical contract source inventory")
	}
	for _, config := range []string{project.Profile, project.ProviderManifest, project.DevManifest} {
		if strings.TrimSpace(config) != "" && relative == clean(config) {
			return decision(relative, "developer-config", MutationEditable, true, "path is an explicit Yunka developer configuration surface")
		}
	}
	if within(relative, project.GeneratedGoRoot) {
		return decision(relative, "developer-code", MutationEditable, true, "path is inside the developer Go root and is not owned or reserved by a canonical generator")
	}

	return decision(relative, "unclassified", MutationUnclassified, false, "no canonical Yunka ownership fact proves this path safe for automatic editing")
}

func render(report Report, format string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", dxoutput.FormatText:
		var builder strings.Builder
		for _, item := range report.Decisions {
			fmt.Fprintf(&builder, "%-15s %-22s %s — %s\n", item.Mutation, item.Owner, item.Path, item.Reason)
		}
		return builder.String(), nil
	case dxoutput.FormatJSON:
		contents, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return "", err
		}
		return string(append(contents, '\n')), nil
	default:
		return "", fmt.Errorf("ownership: unsupported format %q; use text or json", format)
	}
}

func normalizePath(root, path string) (string, bool) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}
	candidate := strings.TrimSpace(path)
	if candidate == "" {
		return "", false
	}
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, filepath.FromSlash(candidate))
	}
	candidate, err = filepath.Abs(candidate)
	if err != nil {
		return "", false
	}
	relative, err := filepath.Rel(root, candidate)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false
	}
	return clean(relative), true
}

func within(path, root string) bool {
	path = clean(path)
	root = clean(root)
	if root == "" || root == "." {
		return false
	}
	return path == root || strings.HasPrefix(path, root+"/")
}

func clean(path string) string {
	path = filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(path))))
	if path == "." {
		return ""
	}
	return strings.TrimPrefix(path, "./")
}

func reservedGeneratedPath(path string) bool {
	base := filepath.Base(filepath.FromSlash(path))
	return strings.HasPrefix(base, "zz_yunka_") || strings.HasSuffix(base, ".pb.go") || base == "domain.json"
}

func generatedByMarker(root, relative string) bool {
	path := filepath.Join(root, filepath.FromSlash(relative))
	info, err := os.Lstat(path)
	if err != nil || info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	buffer := make([]byte, 512)
	count, _ := file.Read(buffer)
	prefix := strings.ToLower(string(buffer[:count]))
	return strings.Contains(prefix, "// code generated by yunka") ||
		strings.Contains(prefix, "// code generated by protoc-gen-go")
}

func pathSet(paths []string) map[string]struct{} {
	result := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		result[clean(path)] = struct{}{}
	}
	return result
}

func decision(path, owner, mutation string, safe bool, reason string) Decision {
	return Decision{Path: path, Owner: owner, Mutation: mutation, SafeAutoEdit: safe, Reason: reason}
}
