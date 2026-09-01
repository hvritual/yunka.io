package projectflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	contractcore "yunka.io/pkg/contract"
	"yunka.io/pkg/fastfeedback"
)

// GenerateWithFastFeedback preserves the canonical Generate path and records
// disposable local evidence only after that full generation succeeds. Evidence
// capture/write failures are intentionally non-blocking.
func GenerateWithFastFeedback(ctx context.Context, options Options) (Report, error) {
	report, err := Generate(ctx, options)
	if err != nil {
		return report, err
	}
	project, resolveErr := resolveProject(options)
	if resolveErr == nil {
		recordFastFeedback(ctx, project)
	}
	return report, nil
}

func recordFastFeedback(ctx context.Context, project resolvedProject) {
	metadata, err := buildFastFeedbackMetadata(ctx, project)
	if err != nil {
		return
	}
	_ = fastfeedback.Write(filepath.Join(project.Root, filepath.FromSlash(fastfeedback.CacheRelativePath)), metadata)
}

func buildFastFeedbackMetadata(ctx context.Context, project resolvedProject) (fastfeedback.Metadata, error) {
	toolchain, err := projectToolchainIdentity(ctx, project)
	if err != nil {
		return fastfeedback.Metadata{}, err
	}
	return buildFastFeedbackMetadataWithIdentity(project, fastfeedback.CurrentEngineIdentity(), toolchain)
}

func buildFastFeedbackMetadataWithIdentity(project resolvedProject, engine fastfeedback.EngineIdentity, toolchain string) (fastfeedback.Metadata, error) {
	inputs, err := fastfeedback.FingerprintRoots(fastFeedbackInputRoots(project))
	if err != nil {
		return fastfeedback.Metadata{}, err
	}
	outputRoots := []fastfeedback.Root{
		{Label: "output.contract", Path: project.ContractOut, Optional: true},
		{Label: "output.generatedGo", Path: project.CodeOut, Optional: true},
		{Label: "output.protobufGoManifest", Path: filepath.Join(project.Root, filepath.FromSlash(protobufGoManifestRelativePath)), Optional: true},
	}
	for index, relative := range protobufGoOutputPaths(project) {
		outputRoots = append(outputRoots, fastfeedback.Root{
			Label: fmt.Sprintf("output.protobufGo.%09d", index),
			Path:  filepath.Join(project.Root, filepath.FromSlash(relative)),
		})
	}
	outputs, err := fastfeedback.FingerprintRoots(outputRoots)
	if err != nil {
		return fastfeedback.Metadata{}, err
	}
	return fastfeedback.NewMetadata(engine, toolchain, inputs, outputs)
}

func fastFeedbackInputRoots(project resolvedProject) []fastfeedback.Root {
	roots := []fastfeedback.Root{
		{Label: "project.profile", Path: filepath.Join(project.Root, ".yunka", "project.json"), Optional: true},
		{Label: "project.goMod", Path: filepath.Join(project.Root, "go.mod"), Optional: true},
		{Label: "module.root", Path: project.ModuleRoot, Optional: true},
	}
	if project.InventoryPath != "" {
		roots = append(roots, fastfeedback.Root{Label: "contract.inventory", Path: project.InventoryPath})
		if inventory, err := contractcore.LoadSourceInventory(project.InventoryPath); err == nil {
			sources := append([]contractcore.SourceSet(nil), inventory.SourceSets...)
			sort.Slice(sources, func(i, j int) bool { return sources[i].Name < sources[j].Name })
			for _, source := range sources {
				roots = append(roots, fastfeedback.Root{
					Label: "contract.source." + source.Name,
					Path:  filepath.Join(project.Root, filepath.FromSlash(source.Root)),
				})
				for index, protoPath := range source.ProtoPaths {
					roots = append(roots, fastfeedback.Root{
						Label: fmt.Sprintf("contract.source.%s.protoPath.%09d", source.Name, index),
						Path:  filepath.Join(project.Root, filepath.FromSlash(protoPath)),
					})
				}
			}
		}
	} else {
		roots = append(roots, fastfeedback.Root{Label: "contract.protoRoot", Path: project.ProtoDir})
	}
	for index, protoPath := range project.AdditionalProtoPaths {
		roots = append(roots, fastfeedback.Root{
			Label: fmt.Sprintf("contract.additionalProtoPath.%09d", index),
			Path:  protoPath,
		})
	}
	return roots
}

func projectToolchainIdentity(ctx context.Context, project resolvedProject) (string, error) {
	protoc, err := protocIdentity(ctx, project.Protoc)
	if err != nil {
		return "", err
	}
	required, err := protobufGoRequired(project)
	if err != nil {
		return "", err
	}
	if !required {
		return protoc, nil
	}
	goPlugin, err := resolveProtobufPlugin(project.Root, "PROTOC_GEN_GO", "protoc-gen-go")
	if err != nil {
		return "", err
	}
	grpcPlugin, err := resolveProtobufPlugin(project.Root, "PROTOC_GEN_GO_GRPC", "protoc-gen-go-grpc")
	if err != nil {
		return "", err
	}
	goIdentity, err := executableIdentity(ctx, "protoc-gen-go", goPlugin)
	if err != nil {
		return "", err
	}
	grpcIdentity, err := executableIdentity(ctx, "protoc-gen-go-grpc", grpcPlugin)
	if err != nil {
		return "", err
	}
	return protoc + "|" + goIdentity + "|" + grpcIdentity, nil
}

func protocIdentity(ctx context.Context, configured string) (string, error) {
	name := strings.TrimSpace(configured)
	if name == "" {
		name = "protoc"
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", err
	}
	return executableIdentity(ctx, "protoc", path)
}

func executableIdentity(ctx context.Context, label, path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	versionCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(versionCtx, path, "--version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s identity: %w", label, err)
	}
	version := strings.TrimSpace(string(output))
	if version == "" {
		return "", fmt.Errorf("%s identity: empty --version output", label)
	}
	return label + ":" + version + ":sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}
