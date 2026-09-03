package contract

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hvritual/yunka.io/pkg/assemblyplan"
)

type AssemblyCompilation struct {
	Plan          assemblyplan.Plan
	PlanJSON      []byte
	GoFiles       []GeneratedAssemblyFile
	ModuleGoFiles []GeneratedAssemblyFile
}

// CompileAssembly is the C10.2 deterministic join point for the AssemblyPlan
// and application/transport structural Go. Call CompileBoundAssembly when the
// compiler also owns explicit generated module Go bindings.
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
	files, err = BindAssemblyCapabilities(plan, files)
	if err != nil {
		return AssemblyCompilation{}, err
	}
	compilation := AssemblyCompilation{Plan: plan, PlanJSON: planJSON, GoFiles: files}
	if err := validateAssemblyCompilation(compilation); err != nil {
		return AssemblyCompilation{}, err
	}
	return compilation, nil
}

// CompileBoundAssembly extends CompileAssembly with canonical compiler-local Go
// bindings discovered from generated module source. The bindings are never
// guessed from module names and are used only to emit explicit catalog wiring.
// C10.3 runtime bootstrap is added only here because NewCatalog is proven to
// exist only after those exact module bindings have been qualified.
func CompileBoundAssembly(manifest Manifest, modules []assemblyplan.ModuleInput, bindings []ModuleBinding, options AssemblyCodeOptions) (AssemblyCompilation, error) {
	compilation, err := CompileAssembly(manifest, modules, options)
	if err != nil {
		return AssemblyCompilation{}, err
	}
	moduleFiles, err := RenderAssemblyModuleCode(compilation.Plan, bindings)
	if err != nil {
		return AssemblyCompilation{}, err
	}
	runtimeFiles, err := BindAssemblyRuntime(manifest, compilation.Plan, compilation.GoFiles)
	if err != nil {
		return AssemblyCompilation{}, err
	}
	runtimeFiles, err = BindAssemblyCapabilityRuntime(compilation.Plan, runtimeFiles)
	if err != nil {
		return AssemblyCompilation{}, err
	}
	compilation.GoFiles = runtimeFiles
	compilation.ModuleGoFiles = moduleFiles
	if err := validateAssemblyCompilation(compilation); err != nil {
		return AssemblyCompilation{}, err
	}
	return compilation, nil
}

func WriteAssemblyCompilation(contractOut, codeRoot string, compilation AssemblyCompilation) error {
	contractOut = strings.TrimSpace(contractOut)
	if contractOut == "" {
		return fmt.Errorf("contract assembly compiler: contract output directory is required")
	}
	if err := validateAssemblyCompilation(compilation); err != nil {
		return err
	}
	if err := os.MkdirAll(contractOut, 0o755); err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(contractOut, AssemblyPlanFilename), compilation.PlanJSON, 0o644); err != nil {
		return err
	}
	if err := WriteAssemblyCode(codeRoot, compilation.GoFiles); err != nil {
		return err
	}
	return WriteAssemblyModuleCode(codeRoot, compilation.ModuleGoFiles)
}

func CheckAssemblyCompilation(contractOut, codeRoot string, compilation AssemblyCompilation) ([]Drift, error) {
	contractOut = strings.TrimSpace(contractOut)
	if contractOut == "" {
		return nil, fmt.Errorf("contract assembly compiler: contract output directory is required")
	}
	if err := validateAssemblyCompilation(compilation); err != nil {
		return nil, err
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
	moduleDrift, err := CheckAssemblyModuleCode(codeRoot, compilation.ModuleGoFiles)
	if err != nil {
		return nil, err
	}
	drift = append(drift, moduleDrift...)
	sort.Slice(drift, func(i, j int) bool { return drift[i].File < drift[j].File })
	return drift, nil
}

func validateAssemblyCompilation(compilation AssemblyCompilation) error {
	canonical, err := assemblyplan.CanonicalJSON(compilation.Plan)
	if err != nil {
		return fmt.Errorf("contract assembly compiler: invalid plan: %w", err)
	}
	if !bytes.Equal(canonical, compilation.PlanJSON) {
		return fmt.Errorf("contract assembly compiler: plan and plan artifact bytes do not match")
	}
	if _, err := assemblyplan.LoadBytes(compilation.PlanJSON); err != nil {
		return fmt.Errorf("contract assembly compiler: invalid plan artifact: %w", err)
	}
	if _, err := assemblyFileMap(compilation.GoFiles); err != nil {
		return err
	}
	if _, err := assemblyModuleFileMap(compilation.ModuleGoFiles); err != nil {
		return err
	}
	return nil
}
