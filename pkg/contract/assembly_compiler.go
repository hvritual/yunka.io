package contract

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"yunka.io/pkg/assemblyplan"
)

type AssemblyCompilation struct {
	Plan      assemblyplan.Plan
	PlanJSON  []byte
	GoFiles   []GeneratedAssemblyFile
}

// CompileAssembly is the C10.2 deterministic join point. Contract facts are
// recompiled into the canonical OperationPlan, the caller supplies the already
// qualified static module snapshot, and both the committed AssemblyPlan and
// typed structural Go are produced from that exact joined fact set.
func CompileAssembly(manifest Manifest, modules []assemblyplan.ModuleInput, options AssemblyCodeOptions) (AssemblyCompilation, error) {
	if modules == nil {
		return AssemblyCompilation{}, fmt.Errorf("contract assembly compiler: qualified module snapshot is required")
	}
	manifest.Normalize()
	operations, err := CompileOperationPlans(manifest)
	if err != nil {
		return AssemblyCompilation{}, fmt.Errorf("contract assembly compiler: operation plan: %w", err)
	}
	plan, err := CompileAssemblyPlan(manifest, operations, modules)
	if err != nil {
		return AssemblyCompilation{}, err
	}
	planJSON, err := assemblyplan.CanonicalJSON(plan)
	if err != nil {
		return AssemblyCompilation{}, err
	}
	files, err := RenderAssemblyCode(manifest, plan, options)
	if err != nil {
		return AssemblyCompilation{}, err
	}
	return AssemblyCompilation{Plan: plan, PlanJSON: planJSON, GoFiles: files}, nil
}

func WriteAssemblyCompilation(contractOut, codeRoot string, compilation AssemblyCompilation) error {
	contractOut = strings.TrimSpace(contractOut)
	if contractOut == "" {
		return fmt.Errorf("contract assembly compiler: contract output directory is required")
	}
	if _, err := assemblyplan.LoadBytes(compilation.PlanJSON); err != nil {
		return fmt.Errorf("contract assembly compiler: invalid plan artifact: %w", err)
	}
	if err := os.MkdirAll(contractOut, 0o755); err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(contractOut, AssemblyPlanFilename), compilation.PlanJSON, 0o644); err != nil {
		return err
	}
	return WriteAssemblyCode(codeRoot, compilation.GoFiles)
}

func CheckAssemblyCompilation(contractOut, codeRoot string, compilation AssemblyCompilation) ([]Drift, error) {
	contractOut = strings.TrimSpace(contractOut)
	if contractOut == "" {
		return nil, fmt.Errorf("contract assembly compiler: contract output directory is required")
	}
	if _, err := assemblyplan.LoadBytes(compilation.PlanJSON); err != nil {
		return nil, fmt.Errorf("contract assembly compiler: invalid plan artifact: %w", err)
	}
	var drift []Drift
	path := filepath.Join(contractOut, AssemblyPlanFilename)
	got, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			drift = append(drift, Drift{File: AssemblyPlanFilename, Reason: "generated assembly plan is missing", Missing: true})
		} else {
			return nil, err
		}
	} else if !bytes.Equal(got, compilation.PlanJSON) {
		drift = append(drift, Drift{File: AssemblyPlanFilename, Reason: "generated assembly plan differs from canonical assembly compilation"})
	}
	codeDrift, err := CheckAssemblyCode(codeRoot, compilation.GoFiles)
	if err != nil {
		return nil, err
	}
	drift = append(drift, codeDrift...)
	sort.Slice(drift, func(i, j int) bool { return drift[i].File < drift[j].File })
	return drift, nil
}
