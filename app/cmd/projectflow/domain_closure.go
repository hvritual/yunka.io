package projectflow

import (
	"context"
	"fmt"

	domaincmd "yunka.io/app/cmd/domain"
)

// generateProjectWithFastFeedback expands the developer happy path from the
// contract/assembly closure to the whole project closure. Domain and standard
// protobuf Go generation run before contract/application/assembly generation so
// fast-feedback evidence is recorded only after every generated artifact is
// current.
func generateProjectWithFastFeedback(ctx context.Context, options Options) (Report, error) {
	project, err := resolveProject(options)
	if err != nil {
		return GenerateWithFastFeedback(ctx, options)
	}
	domainCount, err := domaincmd.RegenerateAll(project.CodeOut)
	if err != nil {
		return Report{}, fmt.Errorf("generate domains: %w", err)
	}
	protobufCount, protobufEnabled, err := generateProtobufGo(ctx, project)
	if err != nil {
		return Report{}, fmt.Errorf("generate protobuf Go: %w", err)
	}
	providerCount, providerEnabled, err := validateProviderClosure(project)
	if err != nil {
		return Report{}, fmt.Errorf("generate providers: %w", err)
	}
	report, err := GenerateWithFastFeedback(ctx, options)
	if err != nil {
		return report, err
	}
	report.Stages = prependProjectClosureStages(report.Stages, project, domainCount, protobufCount, protobufEnabled, providerCount, providerEnabled, "generated")
	return report, nil
}

// checkProject expands the authoritative read-only project check with Domain,
// protobuf Go, and typed infrastructure provider validation. It deliberately
// calls the canonical Check function rather than the fast wrapper to avoid
// recursion.
func checkProject(ctx context.Context, options Options) (Report, error) {
	project, err := resolveProject(options)
	if err != nil {
		return Check(ctx, options)
	}
	domainCount, err := domaincmd.CheckAll(project.CodeOut)
	if err != nil {
		return Report{}, fmt.Errorf("check domains: %w", err)
	}
	protobufCount, protobufEnabled, err := checkProtobufGo(ctx, project)
	if err != nil {
		return Report{}, fmt.Errorf("check protobuf Go: %w", err)
	}
	providerCount, providerEnabled, err := validateProviderClosure(project)
	if err != nil {
		return Report{}, fmt.Errorf("check providers: %w", err)
	}
	report, err := Check(ctx, options)
	if err != nil {
		return report, err
	}
	report.Stages = prependProjectClosureStages(report.Stages, project, domainCount, protobufCount, protobufEnabled, providerCount, providerEnabled, "ok")
	return report, nil
}

func prependProjectClosureStages(stages []Stage, project resolvedProject, domainCount, protobufCount int, protobufEnabled bool, providerCount int, providerEnabled bool, successStatus string) []Stage {
	domainStage := Stage{Name: "domains", Status: successStatus, Detail: fmt.Sprintf("count=%d root=%s", domainCount, relative(project.Root, project.CodeOut))}
	if domainCount == 0 {
		domainStage.Status = "skipped"
		domainStage.Detail = "no managed domains"
	}
	protobufStage := Stage{Name: "protobuf-go", Status: successStatus, Detail: fmt.Sprintf("files=%d", protobufCount)}
	if !protobufEnabled {
		protobufStage.Status = "skipped"
		protobufStage.Detail = "no Go-generating proto sources"
	}
	providerStage := Stage{Name: "providers", Status: successStatus, Detail: fmt.Sprintf("bindings=%d", providerCount)}
	if !providerEnabled {
		providerStage.Status = "skipped"
		providerStage.Detail = "no provider manifest or module capability requirements"
	}
	result := make([]Stage, 0, len(stages)+3)
	result = append(result, domainStage, protobufStage, providerStage)
	return append(result, stages...)
}
