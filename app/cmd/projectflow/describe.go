package projectflow

import (
	"path/filepath"

	projectcmd "yunka.io/app/cmd/project"
)

// ProjectDescriptor is a read-only projection of the same project resolution
// used by the canonical generate/check workflow. It contains locations and
// identity only; it does not duplicate contract, operation, assembly, or
// runtime semantics.
type ProjectDescriptor struct {
	Root               string `json:"-"`
	Profiled           bool   `json:"profiled"`
	Profile            string `json:"profile,omitempty"`
	GoModule           string `json:"goModule,omitempty"`
	ContractSourceKind string `json:"contractSourceKind"`
	ContractSource     string `json:"contractSource"`
	ContractGenerated  string `json:"contractGenerated"`
	ModulesRoot        string `json:"modulesRoot"`
	GeneratedGoRoot    string `json:"generatedGoRoot"`
	GeneratedGoImport  string `json:"generatedGoImport,omitempty"`
	DevManifest        string `json:"devManifest"`
	ProviderManifest   string `json:"providerManifest"`
	ProtobufGoManifest string `json:"protobufGoManifest"`
}

// DescribeProject exposes canonical project resolution to read-only control
// plane commands such as `yunka context`. Keeping the resolver in one place
// prevents AI/developer tooling from creating a second source of truth.
func DescribeProject(options Options) (ProjectDescriptor, error) {
	project, err := resolveProject(options)
	if err != nil {
		return ProjectDescriptor{}, err
	}

	descriptor := ProjectDescriptor{
		Root:               project.Root,
		Profiled:           project.Profiled,
		GoModule:           project.GoModule,
		ContractGenerated:  relative(project.Root, project.ContractOut),
		ModulesRoot:        relative(project.Root, project.ModuleRoot),
		GeneratedGoRoot:    relative(project.Root, project.CodeOut),
		GeneratedGoImport:  project.CodeImport,
		DevManifest:        relative(project.Root, project.DevManifest),
		ProviderManifest:   projectcmd.ProviderManifestRelativePath,
		ProtobufGoManifest: projectcmd.ProtobufGoManifestRelativePath,
	}
	if project.Profiled {
		descriptor.Profile = projectcmd.ConfigRelativePath
	}
	if project.InventoryPath != "" {
		descriptor.ContractSourceKind = "inventory"
		descriptor.ContractSource = relative(project.Root, project.InventoryPath)
	} else {
		descriptor.ContractSourceKind = "proto-root"
		descriptor.ContractSource = relative(project.Root, project.ProtoDir)
	}
	return descriptor, nil
}

// ResolveDescriptorPath resolves a descriptor-relative location against the
// canonical project root. It intentionally performs no discovery or fallback.
func ResolveDescriptorPath(descriptor ProjectDescriptor, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(descriptor.Root, filepath.FromSlash(path))
}
