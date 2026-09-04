package change

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"yunka.io/app/cmd/add"
	"yunka.io/app/cmd/ownership"
	"yunka.io/app/cmd/projectflow"
)

func bindCreateProtobufGoGeneratedPaths(root string, plan add.Report) ([]string, error) {
	source, err := createPlanProtoSource(plan)
	if err != nil {
		return nil, err
	}
	candidates, err := projectflow.ProtobufGoOutputCandidatesForSource(projectflow.Options{Root: root}, source)
	if err != nil {
		return nil, fmt.Errorf("change set begin: resolve protobuf Go outputs for %s: %w", source, err)
	}
	if len(candidates) == 0 {
		return []string{}, nil
	}

	report, err := ownership.Build(root, candidates)
	if err != nil {
		return nil, fmt.Errorf("change set begin: classify protobuf Go outputs for %s: %w", source, err)
	}
	generated := make([]string, 0, len(report.Decisions))
	for _, decision := range report.Decisions {
		if decision.Mutation == ownership.MutationGeneratedOnly {
			generated = append(generated, decision.Path)
			continue
		}

		absolute := filepath.Join(root, filepath.FromSlash(decision.Path))
		if _, statErr := os.Lstat(absolute); statErr == nil {
			return nil, fmt.Errorf("change set begin: protobuf Go output candidate %s exists but AX2 does not prove generator ownership: %s", decision.Path, decision.Reason)
		} else if !os.IsNotExist(statErr) {
			return nil, fmt.Errorf("change set begin: inspect protobuf Go output candidate %s: %w", decision.Path, statErr)
		}
	}
	return uniqueSorted(generated), nil
}

func createPlanProtoSource(plan add.Report) (string, error) {
	var source string
	for _, mutation := range plan.Mutations {
		if mutation.Action != "modified" || !strings.EqualFold(filepath.Ext(strings.TrimSpace(mutation.Path)), ".proto") {
			continue
		}
		if source != "" {
			return "", fmt.Errorf("change set begin: create plan must contain exactly one modified protobuf source")
		}
		source = cleanProjectPath(mutation.Path)
	}
	if source == "" {
		return "", fmt.Errorf("change set begin: create plan modified protobuf source is required")
	}
	return source, nil
}
