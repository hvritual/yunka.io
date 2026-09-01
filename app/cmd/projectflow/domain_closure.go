package projectflow

import (
	"context"
	"fmt"

	domaincmd "yunka.io/app/cmd/domain"
)

// generateProjectWithFastFeedback expands the developer happy path from the
// contract/assembly closure to the whole project closure. Domain generation is
// intentionally performed before contract/application/assembly generation so
// downstream fingerprints are recorded only after all generated Go output is
// current.
func generateProjectWithFastFeedback(ctx context.Context, options Options) (Report, error) {
	project, err := resolveProject(options)
	if err != nil {
		return GenerateWithFastFeedback(ctx, options)
	}
	count, err := domaincmd.RegenerateAll(project.CodeOut)
	if err != nil {
		return Report{}, fmt.Errorf("generate domains: %w", err)
	}
	report, err := GenerateWithFastFeedback(ctx, options)
	if err != nil {
		return report, err
	}
	report.Stages = prependDomainStage(report.Stages, project, count, "generated")
	return report, nil
}

// checkProject expands the authoritative read-only project check with domain
// drift validation. It deliberately calls the canonical Check function rather
// than the fast wrapper to avoid recursion.
func checkProject(ctx context.Context, options Options) (Report, error) {
	project, err := resolveProject(options)
	if err != nil {
		return Check(ctx, options)
	}
	count, err := domaincmd.CheckAll(project.CodeOut)
	if err != nil {
		return Report{}, fmt.Errorf("check domains: %w", err)
	}
	report, err := Check(ctx, options)
	if err != nil {
		return report, err
	}
	report.Stages = prependDomainStage(report.Stages, project, count, "ok")
	return report, nil
}

func prependDomainStage(stages []Stage, project resolvedProject, count int, successStatus string) []Stage {
	stage := Stage{Name: "domains", Status: successStatus, Detail: fmt.Sprintf("count=%d root=%s", count, relative(project.Root, project.CodeOut))}
	if count == 0 {
		stage.Status = "skipped"
		stage.Detail = "no managed domains"
	}
	result := make([]Stage, 0, len(stages)+1)
	result = append(result, stage)
	return append(result, stages...)
}
