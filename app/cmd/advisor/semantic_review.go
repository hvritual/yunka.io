package advisor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/urfave/cli"
	"yunka.io/app/cmd/advisorcore"
	"yunka.io/app/cmd/change"
	"yunka.io/app/cmd/projectflow"
)

const semanticReviewSourceBudget = 2 << 20

func semanticCommand() cli.Command {
	return cli.Command{
		Name:  "semantic",
		Usage: "validate advisory-only semantic architecture findings against exact source/change evidence",
		Subcommands: []cli.Command{
			semanticRequestCommand(),
			semanticValidateCommand(),
			semanticCompareCommand(),
		},
	}
}

func semanticRequestCommand() cli.Command {
	return cli.Command{
		Name:  "request",
		Usage: "export exact source evidence for an external semantic architecture reviewer",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
			cli.StringSliceFlag{Name: "path", Usage: "exact project-relative UTF-8 source file; may be repeated"},
			cli.StringFlag{Name: "review-packet", Usage: "optional exact-candidate review packet to bind this semantic review to a verified change"},
			cli.StringFlag{Name: "change-contract", Value: change.DefaultChangeContractPath, Usage: "change contract used when --review-packet is supplied"},
			cli.StringFlag{Name: "change-attestation", Value: change.DefaultChangeAttestationPath, Usage: "change attestation used when --review-packet is supplied"},
			cli.StringFlag{Name: "protoc", EnvVar: "PROTOC", Usage: "protoc binary used for exact-candidate reconciliation"},
			cli.StringSliceFlag{Name: "proto-path", Usage: "additional protoc import path used for exact-candidate reconciliation; may be repeated"},
			cli.StringFlag{Name: "format", Value: "text", Usage: "output format: text, json, or agent-json"},
		},
		Action: func(c *cli.Context) error {
			request, err := BuildSemanticRequest(context.Background(), projectflow.Options{
				Root:       c.String("root"),
				Protoc:     c.String("protoc"),
				ProtoPaths: c.StringSlice("proto-path"),
			}, c.StringSlice("path"), c.String("review-packet"), c.String("change-contract"), c.String("change-attestation"))
			if err != nil {
				return err
			}
			output, err := RenderSemanticRequest(request, c.String("format"))
			if err != nil {
				return err
			}
			fmt.Print(output)
			return nil
		},
	}
}

func semanticValidateCommand() cli.Command {
	return cli.Command{
		Name:  "validate",
		Usage: "validate a semantic reviewer response and bind every finding to exact evidence",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "request", Usage: "path to semantic review request JSON"},
			cli.StringFlag{Name: "response", Usage: "path to external semantic review response JSON"},
			cli.StringFlag{Name: "format", Value: "text", Usage: "output format: text, json, or agent-json"},
		},
		Action: func(c *cli.Context) error {
			attestation, err := ValidateSemanticFiles(c.String("request"), c.String("response"))
			if err != nil {
				return err
			}
			output, err := RenderSemanticAttestation(attestation, c.String("format"))
			if err != nil {
				return err
			}
			fmt.Print(output)
			return nil
		},
	}
}

func semanticCompareCommand() cli.Command {
	return cli.Command{
		Name:  "compare",
		Usage: "compare two validated semantic review attestations as existing/new/resolved",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "baseline", Usage: "path to baseline semantic review attestation JSON"},
			cli.StringFlag{Name: "current", Usage: "path to current semantic review attestation JSON"},
			cli.StringFlag{Name: "format", Value: "text", Usage: "output format: text, json, or agent-json"},
		},
		Action: func(c *cli.Context) error {
			delta, err := CompareSemanticFiles(c.String("baseline"), c.String("current"))
			if err != nil {
				return err
			}
			output, err := RenderSemanticDelta(delta, c.String("format"))
			if err != nil {
				return err
			}
			fmt.Print(output)
			return nil
		},
	}
}

func BuildSemanticRequest(ctx context.Context, options projectflow.Options, sourcePaths []string, reviewPacketPath, contractPath, attestationPath string) (advisorcore.SemanticReviewRequest, error) {
	if ctx == nil {
		return advisorcore.SemanticReviewRequest{}, fmt.Errorf("advisor semantic request: context is required")
	}
	descriptor, err := projectflow.DescribeProject(options)
	if err != nil {
		return advisorcore.SemanticReviewRequest{}, fmt.Errorf("advisor semantic request: resolve project: %w", err)
	}
	sources, err := readSemanticSources(descriptor.Root, sourcePaths)
	if err != nil {
		return advisorcore.SemanticReviewRequest{}, err
	}
	headSHA, err := gitHead(descriptor.Root)
	if err != nil {
		return advisorcore.SemanticReviewRequest{}, err
	}
	var changeIdentity *advisorcore.SemanticChangeIdentity
	if strings.TrimSpace(reviewPacketPath) != "" {
		currentOptions := options
		currentOptions.Root = descriptor.Root
		packet, err := change.CheckReviewPacket(ctx, currentOptions, contractPath, attestationPath, reviewPacketPath)
		if err != nil {
			return advisorcore.SemanticReviewRequest{}, fmt.Errorf("advisor semantic request: validate review packet: %w", err)
		}
		if packet.Evidence.HeadSHA != headSHA {
			return advisorcore.SemanticReviewRequest{}, fmt.Errorf("advisor semantic request: review packet head %s differs from current HEAD %s", packet.Evidence.HeadSHA, headSHA)
		}
		changeIdentity = &advisorcore.SemanticChangeIdentity{
			BaseSHA:         packet.Evidence.BaseSHA,
			HeadSHA:         packet.Evidence.HeadSHA,
			CandidateSHA256: packet.Evidence.CandidateSHA256,
			EvidenceSHA256:  packet.Evidence.EvidenceSHA256,
		}
	}
	return advisorcore.NewSemanticReviewRequest(headSHA, sources, changeIdentity)
}

func ValidateSemanticFiles(requestPath, responsePath string) (advisorcore.SemanticReviewAttestation, error) {
	requestPath = strings.TrimSpace(requestPath)
	responsePath = strings.TrimSpace(responsePath)
	if requestPath == "" || responsePath == "" {
		return advisorcore.SemanticReviewAttestation{}, fmt.Errorf("advisor semantic validate: --request and --response are required")
	}
	requestBytes, err := os.ReadFile(requestPath)
	if err != nil {
		return advisorcore.SemanticReviewAttestation{}, fmt.Errorf("advisor semantic validate: read request: %w", err)
	}
	responseBytes, err := os.ReadFile(responsePath)
	if err != nil {
		return advisorcore.SemanticReviewAttestation{}, fmt.Errorf("advisor semantic validate: read response: %w", err)
	}
	request, err := advisorcore.DecodeSemanticReviewRequest(requestBytes)
	if err != nil {
		return advisorcore.SemanticReviewAttestation{}, err
	}
	response, err := advisorcore.DecodeSemanticReviewResponse(responseBytes)
	if err != nil {
		return advisorcore.SemanticReviewAttestation{}, err
	}
	return advisorcore.ValidateSemanticReviewResponse(request, response)
}

func CompareSemanticFiles(baselinePath, currentPath string) (advisorcore.SemanticFindingDelta, error) {
	baselinePath = strings.TrimSpace(baselinePath)
	currentPath = strings.TrimSpace(currentPath)
	if baselinePath == "" || currentPath == "" {
		return advisorcore.SemanticFindingDelta{}, fmt.Errorf("advisor semantic compare: --baseline and --current are required")
	}
	baselineBytes, err := os.ReadFile(baselinePath)
	if err != nil {
		return advisorcore.SemanticFindingDelta{}, fmt.Errorf("advisor semantic compare: read baseline: %w", err)
	}
	currentBytes, err := os.ReadFile(currentPath)
	if err != nil {
		return advisorcore.SemanticFindingDelta{}, fmt.Errorf("advisor semantic compare: read current: %w", err)
	}
	baseline, err := advisorcore.DecodeSemanticReviewAttestation(baselineBytes)
	if err != nil {
		return advisorcore.SemanticFindingDelta{}, err
	}
	current, err := advisorcore.DecodeSemanticReviewAttestation(currentBytes)
	if err != nil {
		return advisorcore.SemanticFindingDelta{}, err
	}
	return advisorcore.CompareSemanticReviewAttestations(baseline, current)
}

func RenderSemanticRequest(request advisorcore.SemanticReviewRequest, format string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json", "agent-json":
		contents, err := advisorcore.MarshalSemanticReviewRequest(request)
		if err != nil {
			return "", err
		}
		return string(contents), nil
	case "", "text":
		changeBound := request.Evidence.Change != nil
		return fmt.Sprintf("SEMANTIC REVIEW REQUEST authority=%s request=%s source=%s head=%s files=%d changeBound=%t mutationAuthorized=false mergeAuthorized=false\n", request.Authority, request.RequestDigest, request.Evidence.SourceIdentity, request.Evidence.HeadSHA, len(request.Evidence.Sources), changeBound), nil
	default:
		return "", fmt.Errorf("advisor semantic request: unsupported format %q; use text, json, or agent-json", format)
	}
}

func RenderSemanticAttestation(attestation advisorcore.SemanticReviewAttestation, format string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json", "agent-json":
		contents, err := advisorcore.MarshalSemanticReviewAttestation(attestation)
		if err != nil {
			return "", err
		}
		return string(contents), nil
	case "", "text":
		var builder strings.Builder
		fmt.Fprintf(&builder, "SEMANTIC REVIEW VALID authority=%s source=%s findings=%d result=%s\n", attestation.Authority, attestation.SourceIdentity, len(attestation.Findings), attestation.Result)
		for _, finding := range attestation.Findings {
			fmt.Fprintf(&builder, "[%s] %s %s :: %s\n", finding.Severity, finding.Category, finding.Path, finding.SymbolOrScope)
			fmt.Fprintf(&builder, "  reason: %s\n", finding.Reason)
			fmt.Fprintf(&builder, "  action: %s — %s\n", finding.RecommendedAction.Kind, finding.RecommendedAction.Detail)
			fmt.Fprintf(&builder, "  behavior-change-required: %t\n", finding.BehaviorChangeRequired)
			fmt.Fprintf(&builder, "  finding-id: %s\n", finding.ID)
		}
		return builder.String(), nil
	default:
		return "", fmt.Errorf("advisor semantic validate: unsupported format %q; use text, json, or agent-json", format)
	}
}

func RenderSemanticDelta(delta advisorcore.SemanticFindingDelta, format string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json", "agent-json":
		contents, err := advisorcore.MarshalSemanticFindingDelta(delta)
		if err != nil {
			return "", err
		}
		return string(contents), nil
	case "", "text":
		return fmt.Sprintf("SEMANTIC REVIEW DELTA baseline=%s current=%s existing=%d new=%d resolved=%d digest=%s\n", delta.BaselineSourceIdentity, delta.CurrentSourceIdentity, len(delta.Existing), len(delta.New), len(delta.Resolved), delta.DeltaDigest), nil
	default:
		return "", fmt.Errorf("advisor semantic compare: unsupported format %q; use text, json, or agent-json", format)
	}
}

func readSemanticSources(root string, sourcePaths []string) ([]advisorcore.SemanticSource, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("advisor semantic request: resolve project root: %w", err)
	}
	if len(sourcePaths) == 0 {
		return nil, fmt.Errorf("advisor semantic request: at least one --path is required")
	}
	seen := map[string]struct{}{}
	sources := make([]advisorcore.SemanticSource, 0, len(sourcePaths))
	total := 0
	for _, input := range sourcePaths {
		input = strings.TrimSpace(input)
		if input == "" || filepath.IsAbs(filepath.FromSlash(input)) {
			return nil, fmt.Errorf("advisor semantic request: source path %q must be project-relative", input)
		}
		absolute := filepath.Clean(filepath.Join(absoluteRoot, filepath.FromSlash(input)))
		relative, err := filepath.Rel(absoluteRoot, absolute)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
			return nil, fmt.Errorf("advisor semantic request: source path %q escapes project root", input)
		}
		projectPath := filepath.ToSlash(relative)
		if _, duplicate := seen[projectPath]; duplicate {
			return nil, fmt.Errorf("advisor semantic request: duplicate source path %q", projectPath)
		}
		seen[projectPath] = struct{}{}
		info, err := os.Lstat(absolute)
		if err != nil {
			return nil, fmt.Errorf("advisor semantic request: stat %s: %w", projectPath, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("advisor semantic request: source %s must be a regular non-symlink file", projectPath)
		}
		contents, err := os.ReadFile(absolute)
		if err != nil {
			return nil, fmt.Errorf("advisor semantic request: read %s: %w", projectPath, err)
		}
		if !utf8.Valid(contents) {
			return nil, fmt.Errorf("advisor semantic request: source %s is not UTF-8 text", projectPath)
		}
		total += len(contents)
		if total > semanticReviewSourceBudget {
			return nil, fmt.Errorf("advisor semantic request: exact source evidence exceeds %d bytes", semanticReviewSourceBudget)
		}
		sources = append(sources, advisorcore.SemanticSource{Path: projectPath, Content: string(contents)})
	}
	return sources, nil
}

func gitHead(root string) (string, error) {
	command := exec.Command("git", "-C", root, "rev-parse", "HEAD")
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("advisor semantic request: resolve HEAD: %w: %s", err, strings.TrimSpace(string(output)))
	}
	value := strings.TrimSpace(string(output))
	if value == "" {
		return "", fmt.Errorf("advisor semantic request: Git returned an empty HEAD")
	}
	return value, nil
}
