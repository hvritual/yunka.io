package projectflow

import (
	"context"
	"path/filepath"

	"yunka.io/pkg/fastfeedback"
)

type canonicalGenerateFunc func(context.Context, Options) (Report, error)

// GenerateIncremental is the top-level developer happy-path wrapper. It may
// return a no-op success only when the qualified C11.4 evidence proves the
// running engine, complete protobuf toolchain, canonical inputs, and existing
// generated outputs are an exact reusable match. Every other state falls back
// to the full project generation path.
func GenerateIncremental(ctx context.Context, options Options, forceFull bool) (Report, error) {
	identity := toolchainIdentityFunc(protocIdentity)
	if project, err := resolveProject(options); err == nil {
		identity = func(ctx context.Context, _ string) (string, error) {
			return projectToolchainIdentity(ctx, project)
		}
	}
	return generateIncremental(
		ctx,
		options,
		forceFull,
		fastfeedback.CurrentEngineIdentity(),
		identity,
		generateProjectWithFastFeedback,
	)
}

func generateIncremental(
	ctx context.Context,
	options Options,
	forceFull bool,
	engine fastfeedback.EngineIdentity,
	toolchainIdentity toolchainIdentityFunc,
	fullGenerate canonicalGenerateFunc,
) (Report, error) {
	if forceFull {
		return fullGenerate(ctx, options)
	}
	project, err := resolveProject(options)
	if err != nil {
		return fullGenerate(ctx, options)
	}
	cachePath := filepath.Join(project.Root, filepath.FromSlash(fastfeedback.CacheRelativePath))
	cached, err := fastfeedback.Load(cachePath)
	if err != nil {
		return fullGenerate(ctx, options)
	}
	toolchain, err := toolchainIdentity(ctx, project.Protoc)
	if err != nil {
		return fullGenerate(ctx, options)
	}
	current, err := buildFastFeedbackMetadataWithIdentity(project, engine, toolchain)
	if err != nil || !fastfeedback.Reusable(cached, current) {
		return fullGenerate(ctx, options)
	}
	return Report{
		Root: project.Root,
		Stages: []Stage{{
			Name:   "fast-generate",
			Status: "unchanged",
			Detail: "verified engine/toolchain/input/output fingerprints",
		}},
	}, nil
}
