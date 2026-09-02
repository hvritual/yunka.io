package projectflow

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	modulecmd "yunka.io/app/cmd/module"
	projectcmd "yunka.io/app/cmd/project"
	contractcore "github.com/hvritual/yunka.io/pkg/contract"
)

const (
	defaultContractTitle   = "yunka API"
	defaultContractVersion = "1.0.0"
)

type Options struct {
	Root       string
	Protoc     string
	ProtoPaths []string
}

type Stage struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type Report struct {
	Root   string  `json:"-"`
	Stages []Stage `json:"stages"`
}

type resolvedProject struct {
	Root                 string
	InventoryPath        string
	ProtoDir             string
	ContractOut          string
	ModuleRoot           string
	CodeOut              string
	GoModule             string
	CodeImport           string
	DevManifest          string
	Profiled             bool
	Protoc               string
	AdditionalProtoPaths []string
}

func Generate(ctx context.Context, options Options) (Report, error) {
	failureRoot := workflowRoot(options.Root)
	project, err := resolveProject(options)
	if err != nil {
		return Report{}, wrapFailure(FailureProject, failureRoot, "", err)
	}
	result, artifacts, applicationFiles, err := compileArtifacts(ctx, project)
	if err != nil {
		return Report{}, wrapFailure(FailureContract, project.Root, "", fmt.Errorf("generate contract: %w", err))
	}
	if err := contractcore.WriteArtifacts(project.ContractOut, artifacts); err != nil {
		return Report{}, wrapFailure(FailureContract, project.Root, relative(project.Root, project.ContractOut), fmt.Errorf("generate contract artifacts: %w", err))
	}
	if len(applicationFiles) > 0 {
		if err := contractcore.WriteApplicationCode(project.CodeOut, applicationFiles); err != nil {
			return Report{}, wrapFailure(FailureContract, project.Root, relative(project.Root, project.CodeOut), fmt.Errorf("generate application code: %w", err))
		}
	}

	report := Report{Root: project.Root}
	report.Stages = append(report.Stages, Stage{
		Name:   "contract",
		Status: "generated",
		Detail: fmt.Sprintf("services=%d messages=%d applicationFiles=%d out=%s", len(result.Manifest.Services), len(result.Manifest.Messages), len(applicationFiles), relative(project.Root, project.ContractOut)),
	})

	if err := modulecmd.Check(project.ModuleRoot); err != nil {
		return Report{}, wrapFailure(FailureModule, project.Root, relative(project.Root, project.ModuleRoot), fmt.Errorf("generate module check: %w", err))
	}
	modulesPresent, err := hasModules(project.ModuleRoot)
	if err != nil {
		return Report{}, wrapFailure(FailureModule, project.Root, relative(project.Root, project.ModuleRoot), fmt.Errorf("generate module discovery: %w", err))
	}
	if modulesPresent {
		report.Stages = append(report.Stages, Stage{Name: "modules", Status: "checked", Detail: relative(project.Root, project.ModuleRoot)})
	} else {
		report.Stages = append(report.Stages, Stage{Name: "modules", Status: "skipped", Detail: "no generated modules"})
	}

	assemblyRequired := modulesPresent || contractcore.HasTypedApplications(result.Manifest)
	if !assemblyRequired {
		report.Stages = append(report.Stages, Stage{Name: "assembly", Status: "skipped", Detail: "no typed applications or generated modules"})
		return report, nil
	}
	if strings.TrimSpace(project.CodeImport) == "" {
		return Report{}, wrapFailure(FailureAssembly, project.Root, ".yunka/project.json", errors.New("generate assembly: generated Go import root is unavailable; add go.mod or workflow.generatedGo.import to .yunka/project.json"))
	}
	compilation, bindingCount, err := compileAssembly(result.Manifest, project)
	if err != nil {
		return Report{}, wrapFailure(FailureAssembly, project.Root, relative(project.Root, project.CodeOut), fmt.Errorf("generate assembly: %w", err))
	}
	if err := contractcore.WriteAssemblyCompilation(project.ContractOut, project.CodeOut, compilation); err != nil {
		return Report{}, wrapFailure(FailureAssembly, project.Root, relative(project.Root, project.CodeOut), fmt.Errorf("generate assembly artifacts: %w", err))
	}
	report.Stages = append(report.Stages, Stage{
		Name:   "assembly",
		Status: "generated",
		Detail: fmt.Sprintf("bindings=%d codeOut=%s", bindingCount, relative(project.Root, project.CodeOut)),
	})
	return report, nil
}

func Check(ctx context.Context, options Options) (Report, error) {
	failureRoot := workflowRoot(options.Root)
	project, err := resolveProject(options)
	if err != nil {
		return Report{}, wrapFailure(FailureProject, failureRoot, "", err)
	}
	result, artifacts, applicationFiles, err := compileArtifacts(ctx, project)
	if err != nil {
		return Report{}, wrapFailure(FailureContract, project.Root, "", fmt.Errorf("check contract: %w", err))
	}
	contractDrift, err := contractcore.CheckArtifacts(project.ContractOut, artifacts)
	if err != nil {
		return Report{}, wrapFailure(FailureContract, project.Root, relative(project.Root, project.ContractOut), fmt.Errorf("check contract artifacts: %w", err))
	}
	if len(contractDrift) > 0 {
		first := contractDrift[0]
		location := filepath.ToSlash(filepath.Join(relative(project.Root, project.ContractOut), filepath.FromSlash(first.File)))
		return Report{}, wrapFailure(FailureContractDrift, project.Root, location, fmt.Errorf("check contract: generated artifacts are stale (%s: %s); run `yunka generate`", first.File, first.Reason))
	}
	if len(applicationFiles) > 0 {
		applicationDrift, err := contractcore.CheckApplicationCode(project.CodeOut, applicationFiles)
		if err != nil {
			return Report{}, wrapFailure(FailureContract, project.Root, relative(project.Root, project.CodeOut), fmt.Errorf("check application code: %w", err))
		}
		if len(applicationDrift) > 0 {
			first := applicationDrift[0]
			location := filepath.ToSlash(filepath.Join(relative(project.Root, project.CodeOut), filepath.FromSlash(first.File)))
			return Report{}, wrapFailure(FailureContractDrift, project.Root, location, fmt.Errorf("check application code: generated artifacts are stale (%s: %s); run `yunka generate`", first.File, first.Reason))
		}
	}

	report := Report{Root: project.Root}
	report.Stages = append(report.Stages, Stage{
		Name:   "contract",
		Status: "ok",
		Detail: fmt.Sprintf("services=%d messages=%d applicationFiles=%d", len(result.Manifest.Services), len(result.Manifest.Messages), len(applicationFiles)),
	})

	if err := modulecmd.Check(project.ModuleRoot); err != nil {
		return Report{}, wrapFailure(FailureModule, project.Root, relative(project.Root, project.ModuleRoot), fmt.Errorf("check modules: %w", err))
	}
	modulesPresent, err := hasModules(project.ModuleRoot)
	if err != nil {
		return Report{}, wrapFailure(FailureModule, project.Root, relative(project.Root, project.ModuleRoot), fmt.Errorf("check module discovery: %w", err))
	}
	if modulesPresent {
		report.Stages = append(report.Stages, Stage{Name: "modules", Status: "ok", Detail: relative(project.Root, project.ModuleRoot)})
	} else {
		report.Stages = append(report.Stages, Stage{Name: "modules", Status: "skipped", Detail: "no generated modules"})
	}

	assemblyRequired := modulesPresent || contractcore.HasTypedApplications(result.Manifest)
	if !assemblyRequired {
		report.Stages = append(report.Stages, Stage{Name: "assembly", Status: "skipped", Detail: "no typed applications or generated modules"})
		return report, nil
	}
	if strings.TrimSpace(project.CodeImport) == "" {
		return Report{}, wrapFailure(FailureAssembly, project.Root, ".yunka/project.json", errors.New("check assembly: generated Go import root is unavailable; add go.mod or workflow.generatedGo.import to .yunka/project.json"))
	}
	compilation, bindingCount, err := compileAssembly(result.Manifest, project)
	if err != nil {
		return Report{}, wrapFailure(FailureAssembly, project.Root, relative(project.Root, project.CodeOut), fmt.Errorf("check assembly: %w", err))
	}
	assemblyDrift, err := contractcore.CheckAssemblyCompilation(project.ContractOut, project.CodeOut, compilation)
	if err != nil {
		return Report{}, wrapFailure(FailureAssembly, project.Root, relative(project.Root, project.CodeOut), fmt.Errorf("check assembly artifacts: %w", err))
	}
	if len(assemblyDrift) > 0 {
		first := assemblyDrift[0]
		return Report{}, wrapFailure(FailureAssemblyDrift, project.Root, first.File, fmt.Errorf("check assembly: generated artifacts are stale (%s: %s); run `yunka generate`", first.File, first.Reason))
	}
	report.Stages = append(report.Stages, Stage{Name: "assembly", Status: "ok", Detail: fmt.Sprintf("bindings=%d", bindingCount)})
	return report, nil
}

func Format(report Report) string {
	var builder strings.Builder
	for _, stage := range report.Stages {
		fmt.Fprintf(&builder, "%-9s %-9s %s\n", strings.ToUpper(stage.Status), stage.Name, stage.Detail)
	}
	return builder.String()
}

func compileArtifacts(ctx context.Context, project resolvedProject) (contractcore.CompileResult, contractcore.Artifacts, []contractcore.GeneratedApplicationFile, error) {
	result, err := compileContract(ctx, project)
	if err != nil {
		return contractcore.CompileResult{}, contractcore.Artifacts{}, nil, err
	}
	diagnostics := contractcore.Lint(result.Manifest)
	if contractcore.HasErrors(diagnostics) {
		for _, diagnostic := range diagnostics {
			if strings.EqualFold(string(diagnostic.Severity), "error") {
				return contractcore.CompileResult{}, contractcore.Artifacts{}, nil, fmt.Errorf("contract lint %s: %s", diagnostic.Path, diagnostic.Message)
			}
		}
		return contractcore.CompileResult{}, contractcore.Artifacts{}, nil, errors.New("contract lint failed")
	}
	artifacts, err := contractcore.RenderArtifacts(result.Manifest, contractcore.ArtifactOptions{
		OpenAPI: contractcore.OpenAPIOptions{Title: defaultContractTitle, Version: defaultContractVersion},
	})
	if err != nil {
		return contractcore.CompileResult{}, contractcore.Artifacts{}, nil, err
	}
	var applicationFiles []contractcore.GeneratedApplicationFile
	if contractcore.HasTypedApplications(result.Manifest) {
		if strings.TrimSpace(project.CodeImport) == "" {
			return contractcore.CompileResult{}, contractcore.Artifacts{}, nil, errors.New("typed applications require a generated Go import root")
		}
		applicationFiles, err = contractcore.RenderC9ApplicationCode(result.Manifest, contractcore.ApplicationCodeOptions{RootImport: project.CodeImport})
		if err != nil {
			return contractcore.CompileResult{}, contractcore.Artifacts{}, nil, err
		}
	}
	return result, artifacts, applicationFiles, nil
}

func compileContract(ctx context.Context, project resolvedProject) (contractcore.CompileResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	compileCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if project.InventoryPath != "" {
		return contractcore.CompileInventory(compileCtx, contractcore.InventoryCompileOptions{
			RepositoryRoot: project.Root,
			InventoryPath:  project.InventoryPath,
			Protoc:         project.Protoc,
		})
	}
	return contractcore.Compile(compileCtx, contractcore.CompileOptions{
		Dir:        project.ProtoDir,
		ProtoPaths: project.AdditionalProtoPaths,
		Protoc:     project.Protoc,
	})
}

func compileAssembly(manifest contractcore.Manifest, project resolvedProject) (contractcore.AssemblyCompilation, int, error) {
	modules, bindings, err := contractcore.DiscoverModuleSnapshot(project.ModuleRoot)
	if err != nil {
		return contractcore.AssemblyCompilation{}, 0, err
	}
	compilation, err := contractcore.CompileBoundAssembly(manifest, modules, bindings, contractcore.AssemblyCodeOptions{RootImport: project.CodeImport})
	if err != nil {
		return contractcore.AssemblyCompilation{}, 0, err
	}
	return compilation, len(bindings), nil
}

func resolveProject(options Options) (resolvedProject, error) {
	root := strings.TrimSpace(options.Root)
	if root == "" {
		root = "."
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return resolvedProject{}, err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return resolvedProject{}, err
	}
	if !info.IsDir() {
		return resolvedProject{}, fmt.Errorf("project root %s is not a directory", absolute)
	}

	protoc := strings.TrimSpace(options.Protoc)
	if protoc == "" {
		protoc = strings.TrimSpace(os.Getenv("PROTOC"))
	}
	protoPaths := resolveProtoPaths(absolute, options.ProtoPaths)

	config, profileErr := projectcmd.Load(absolute)
	if profileErr == nil {
		return resolveProfileProject(absolute, config, protoc, protoPaths)
	}
	if !errors.Is(profileErr, os.ErrNotExist) {
		return resolvedProject{}, fmt.Errorf("project profile: %w", profileErr)
	}
	return resolveConventionalProject(absolute, protoc, protoPaths)
}

func resolveProfileProject(root string, config projectcmd.Config, protoc string, protoPaths []string) (resolvedProject, error) {
	profile := config.Workflow
	inventoryPath := ""
	protoDir := ""
	if profile.Contract.Sources != "" {
		inventoryPath = profilePath(root, profile.Contract.Sources)
		if err := requireProfileFile("workflow.contract.sources", inventoryPath); err != nil {
			return resolvedProject{}, err
		}
	} else {
		protoDir = profilePath(root, profile.Contract.ProtoRoot)
		if err := requireProfileDir("workflow.contract.protoRoot", protoDir); err != nil {
			return resolvedProject{}, err
		}
	}

	contractOut := profilePath(root, profile.Contract.Generated)
	moduleRoot := profilePath(root, profile.Modules.Root)
	codeOut := profilePath(root, profile.GeneratedGo.Root)
	devManifest := profilePath(root, profile.Dev.Manifest)

	goModule, goModuleErr := readGoModule(filepath.Join(root, "go.mod"))
	if goModuleErr != nil && !errors.Is(goModuleErr, os.ErrNotExist) {
		return resolvedProject{}, fmt.Errorf("project profile go.mod: %w", goModuleErr)
	}
	explicitImport := strings.Trim(strings.TrimSpace(profile.GeneratedGo.Import), "/")
	codeImport := explicitImport
	if goModule != "" {
		derived, err := deriveCodeImport(goModule, root, codeOut)
		if err != nil {
			return resolvedProject{}, err
		}
		if explicitImport != "" && explicitImport != derived {
			return resolvedProject{}, fmt.Errorf("project profile workflow.generatedGo.import %q conflicts with go.mod-derived import %q", explicitImport, derived)
		}
		codeImport = derived
	}

	return resolvedProject{
		Root:                 root,
		InventoryPath:        inventoryPath,
		ProtoDir:             protoDir,
		ContractOut:          contractOut,
		ModuleRoot:           moduleRoot,
		CodeOut:              codeOut,
		GoModule:             goModule,
		CodeImport:           codeImport,
		DevManifest:          devManifest,
		Profiled:             true,
		Protoc:               protoc,
		AdditionalProtoPaths: protoPaths,
	}, nil
}

func resolveConventionalProject(root, protoc string, protoPaths []string) (resolvedProject, error) {
	inventoryPath := filepath.Join(root, "contracts", "sources.json")
	if _, err := os.Stat(inventoryPath); os.IsNotExist(err) {
		inventoryPath = ""
	} else if err != nil {
		return resolvedProject{}, err
	}
	protoDir := filepath.Join(root, "contracts", "proto")
	if inventoryPath == "" {
		if info, err := os.Stat(protoDir); err != nil {
			if os.IsNotExist(err) {
				return resolvedProject{}, errors.New("project has no .yunka/project.json, contracts/sources.json, or contracts/proto directory")
			}
			return resolvedProject{}, err
		} else if !info.IsDir() {
			return resolvedProject{}, fmt.Errorf("contract proto root %s is not a directory", protoDir)
		}
	}

	goModule, err := readGoModule(filepath.Join(root, "go.mod"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return resolvedProject{}, err
	}
	codeOut := filepath.Join(root, "internal")
	codeImport := ""
	if goModule != "" {
		codeImport, err = deriveCodeImport(goModule, root, codeOut)
		if err != nil {
			return resolvedProject{}, err
		}
	}

	return resolvedProject{
		Root:                 root,
		InventoryPath:        inventoryPath,
		ProtoDir:             protoDir,
		ContractOut:          filepath.Join(root, "contracts", "generated"),
		ModuleRoot:           filepath.Join(root, "modules"),
		CodeOut:              codeOut,
		GoModule:             goModule,
		CodeImport:           codeImport,
		DevManifest:          filepath.Join(root, ".yunka", "dev.json"),
		Profiled:             false,
		Protoc:               protoc,
		AdditionalProtoPaths: protoPaths,
	}, nil
}

func resolveProtoPaths(root string, values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if !filepath.IsAbs(value) {
			value = filepath.Join(root, value)
		}
		result = append(result, filepath.Clean(value))
	}
	return result
}

func profilePath(root, value string) string {
	return filepath.Join(root, filepath.FromSlash(value))
}

func requireProfileFile(name, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("project profile %s points to missing file %s", name, path)
		}
		return fmt.Errorf("project profile %s: %w", name, err)
	}
	if info.IsDir() {
		return fmt.Errorf("project profile %s must point to a file, got directory %s", name, path)
	}
	return nil
}

func requireProfileDir(name, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("project profile %s points to missing directory %s", name, path)
		}
		return fmt.Errorf("project profile %s: %w", name, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("project profile %s must point to a directory, got file %s", name, path)
	}
	return nil
}

func deriveCodeImport(goModule, root, codeOut string) (string, error) {
	relative, err := filepath.Rel(root, codeOut)
	if err != nil {
		return "", err
	}
	if relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("generated Go root %s cannot be derived from project go.mod; configure workflow.generatedGo.import explicitly", codeOut)
	}
	return strings.TrimRight(goModule, "/") + "/" + filepath.ToSlash(relative), nil
}

func readGoModule(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			module := strings.TrimSpace(strings.TrimPrefix(line, "module "))
			if module == "" {
				return "", fmt.Errorf("go.mod %s contains an empty module directive", path)
			}
			return module, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("go.mod %s contains no module directive", path)
}

func hasModules(root string) (bool, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			return true, nil
		}
	}
	return false, nil
}

func relative(root, path string) string {
	value, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(value)
}

func workflowRoot(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "."
	}
	if absolute, err := filepath.Abs(root); err == nil {
		return absolute
	}
	return root
}
