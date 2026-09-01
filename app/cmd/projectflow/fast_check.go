package projectflow

import (
	"context"
	"path/filepath"

	"yunka.io/pkg/fastfeedback"
)

type toolchainIdentityFunc func(context.Context, string) (string, error)
type canonicalCheckFunc func(context.Context, Options) (Report, error)

// CheckWithFastFeedback is the developer happy-path wrapper. The canonical
// project closure check remains the authoritative full validation path and is
// always used when fast-feedback evidence is unavailable, invalid,
// unverifiable, or mismatched.
func CheckWithFastFeedback(ctx context.Context, options Options, forceFull bool) (Report, error) {
	return checkWithFastFeedback(
		ctx,
		options,
		forceFull,
		fastfeedback.CurrentEngineIdentity(),
		protocIdentity,
		checkProject,
	)
}

func checkWithFastFeedback(
	ctx context.Context,
	options Options,
	forceFull bool,
	engine fastfeedback.EngineIdentity,
	toolchainIdentity toolchainIdentityFunc,
	fullCheck canonicalCheckFunc,
) (Report, error) {
	if forceFull {
		return fullCheck(ctx, options)
	}
	project, err := resolveProject(options)
	if err != nil {
		return fullCheck(ctx, options)
	}
	cachePath := filepath.Join(project.Root, filepath.FromSlash(fastfeedback.CacheRelativePath))
	cached, err := fastfeedback.Load(cachePath)
	if err != nil {
		return fullCheck(ctx, options)
	}
	toolchain, err := toolchainIdentity(ctx, project.Protoc)
	if err != nil {
		return fullCheck(ctx, options)
	}
	current, err := buildFastFeedbackMetadataWithIdentity(project, engine, toolchain)
	if err != nil || !fastfeedback.Reusable(cached, current) {
		return fullCheck(ctx, options)
	}
	return Report{
		Root: project.Root,
		Stages: []Stage{{
			Name:   "fast-check",
			Status: "ok",
			Detail: "verified engine/toolchain/input/output fingerprints",
		}},
	}, nil
}
