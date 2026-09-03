package diagnostic

import (
	"fmt"
	"sort"
	"strings"
)

const (
	CodeUnsupportedOutputFormat = "YUNKA-DX-DEV-001"
	CodeDeveloperWorkflowFailed = "YUNKA-DX-DEV-999"

	CodeProjectResolve   = "YUNKA-DX-PROJECT-001"
	CodeContractFailure = "YUNKA-DX-CONTRACT-001"
	CodeContractDrift   = "YUNKA-DX-CONTRACT-002"
	CodeModuleFailure   = "YUNKA-DX-MODULE-001"
	CodeAssemblyFailure = "YUNKA-DX-ASSEMBLY-001"
	CodeAssemblyDrift   = "YUNKA-DX-ASSEMBLY-002"

	CodeChangeOperation = "YUNKA-DX-CHANGE-001"
	CodeChangeIntent    = "YUNKA-DX-CHANGE-002"
	CodeChangeEvidence  = "YUNKA-DX-CHANGE-003"

	CodeDoctorWorkspaceRoot    = "YUNKA-DX-PROJECT-101"
	CodeDoctorGoWork           = "YUNKA-DX-TOOLCHAIN-101"
	CodeDoctorToolchainLock    = "YUNKA-DX-TOOLCHAIN-102"
	CodeDoctorGo               = "YUNKA-DX-TOOLCHAIN-110"
	CodeDoctorProtoc           = "YUNKA-DX-TOOLCHAIN-111"
	CodeDoctorProtocGenGo      = "YUNKA-DX-TOOLCHAIN-112"
	CodeDoctorProtocGenGoGRPC  = "YUNKA-DX-TOOLCHAIN-113"
	CodeDoctorGCC              = "YUNKA-DX-TOOLCHAIN-114"
	CodeDoctorGit              = "YUNKA-DX-TOOLCHAIN-115"
	CodeDoctorContractManifest = "YUNKA-DX-CONTRACT-101"
	CodeDoctorContractGraph    = "YUNKA-DX-CONTRACT-102"
	CodeDoctorGitStatus        = "YUNKA-DX-DEV-101"
	CodeDoctorDevManifest      = "YUNKA-DX-DEV-102"
)

// Definition is the stable, dependency-neutral identity behind one developer
// diagnostic code. Dynamic facts remain with the validator/adapter that emits
// the diagnostic; this catalog owns only durable discovery metadata.
type Definition struct {
	Code     string   `json:"code"`
	Stage    string   `json:"stage"`
	Meaning  string   `json:"meaning"`
	Location string   `json:"location,omitempty"`
	Actions  []Action `json:"actions,omitempty"`
}

var definitionCatalog = map[string]Definition{
	CodeUnsupportedOutputFormat: {Code: CodeUnsupportedOutputFormat, Stage: "cli", Meaning: "unsupported output format"},
	CodeDeveloperWorkflowFailed: {Code: CodeDeveloperWorkflowFailed, Stage: "developer-workflow", Meaning: "developer workflow failed"},
	CodeProjectResolve: {
		Code: CodeProjectResolve, Stage: "project", Meaning: "project configuration could not be resolved",
		Actions: []Action{{Kind: ActionEdit, Label: "Review project profile", Value: ".yunka/project.json"}},
	},
	CodeContractFailure: {Code: CodeContractFailure, Stage: "contract", Meaning: "contract generation or validation failed"},
	CodeContractDrift: {
		Code: CodeContractDrift, Stage: "contract", Meaning: "generated contract artifacts are stale",
		Actions: []Action{{Kind: ActionCommand, Label: "Regenerate", Value: "yunka generate"}},
	},
	CodeModuleFailure: {
		Code: CodeModuleFailure, Stage: "module", Meaning: "module validation failed",
		Actions: []Action{{Kind: ActionCommand, Label: "Inspect modules", Value: "yunka module check"}},
	},
	CodeAssemblyFailure: {Code: CodeAssemblyFailure, Stage: "assembly", Meaning: "runtime assembly generation or validation failed"},
	CodeAssemblyDrift: {
		Code: CodeAssemblyDrift, Stage: "assembly", Meaning: "generated runtime assembly artifacts are stale",
		Actions: []Action{{Kind: ActionCommand, Label: "Regenerate", Value: "yunka generate"}},
	},
	CodeChangeOperation: {
		Code: CodeChangeOperation, Stage: "change-planning", Meaning: "requested operation cannot be resolved to one existing canonical operation",
		Actions: []Action{{Kind: ActionCommand, Label: "Inspect operations", Value: "yunka graph find --kind operation"}},
	},
	CodeChangeIntent: {
		Code: CodeChangeIntent, Stage: "change-planning", Meaning: "change intent is missing or unsupported",
	},
	CodeChangeEvidence: {
		Code: CodeChangeEvidence, Stage: "change-planning", Meaning: "canonical evidence required to build a change plan is unavailable or invalid",
		Actions: []Action{{Kind: ActionCommand, Label: "Regenerate canonical facts", Value: "yunka generate"}, {Kind: ActionCommand, Label: "Validate canonical facts", Value: "yunka check --format agent-json"}},
	},
	CodeDoctorWorkspaceRoot:    {Code: CodeDoctorWorkspaceRoot, Stage: "project", Meaning: "workspace root check reported a developer-environment issue"},
	CodeDoctorGoWork:           {Code: CodeDoctorGoWork, Stage: "toolchain", Meaning: "Go workspace configuration check reported an issue", Location: "go.work"},
	CodeDoctorToolchainLock:    {Code: CodeDoctorToolchainLock, Stage: "toolchain", Meaning: "locked toolchain configuration check reported an issue", Location: "tools/toolchain.env"},
	CodeDoctorGo:               {Code: CodeDoctorGo, Stage: "toolchain", Meaning: "Go tool availability or version check reported an issue"},
	CodeDoctorProtoc:           {Code: CodeDoctorProtoc, Stage: "toolchain", Meaning: "protoc availability or version check reported an issue"},
	CodeDoctorProtocGenGo:      {Code: CodeDoctorProtocGenGo, Stage: "toolchain", Meaning: "protoc-gen-go availability or version check reported an issue"},
	CodeDoctorProtocGenGoGRPC:  {Code: CodeDoctorProtocGenGoGRPC, Stage: "toolchain", Meaning: "protoc-gen-go-grpc availability or version check reported an issue"},
	CodeDoctorGCC:              {Code: CodeDoctorGCC, Stage: "toolchain", Meaning: "C compiler availability check reported an issue"},
	CodeDoctorGit:              {Code: CodeDoctorGit, Stage: "toolchain", Meaning: "Git availability check reported an issue"},
	CodeDoctorContractManifest: {Code: CodeDoctorContractManifest, Stage: "contract", Meaning: "generated contract manifest check reported an issue", Location: "contracts/generated/manifest.json"},
	CodeDoctorContractGraph:    {Code: CodeDoctorContractGraph, Stage: "contract", Meaning: "application graph contract evidence check reported an issue", Location: "contracts/generated/manifest.json"},
	CodeDoctorGitStatus:        {Code: CodeDoctorGitStatus, Stage: "developer-environment", Meaning: "Git worktree status check reported an issue"},
	CodeDoctorDevManifest:      {Code: CodeDoctorDevManifest, Stage: "developer-environment", Meaning: "developer runtime manifest check reported an issue", Location: ".yunka/dev.json"},
}

func LookupDefinition(code string) (Definition, bool) {
	code = strings.ToUpper(strings.TrimSpace(code))
	definition, ok := definitionCatalog[code]
	if !ok {
		return Definition{}, false
	}
	return cloneDefinition(definition), true
}

func MustDefinition(code string) Definition {
	definition, ok := LookupDefinition(code)
	if !ok {
		panic(fmt.Sprintf("diagnostic: definition %q is not registered", code))
	}
	return definition
}

func Definitions() []Definition {
	result := make([]Definition, 0, len(definitionCatalog))
	for _, definition := range definitionCatalog {
		result = append(result, cloneDefinition(definition))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Code < result[j].Code })
	return result
}

func (definition Definition) Diagnostic(severity Severity) Diagnostic {
	item := Diagnostic{
		Code:     definition.Code,
		Severity: severity,
		Stage:    definition.Stage,
		Summary:  definition.Meaning,
		Actions:  append([]Action(nil), definition.Actions...),
	}
	if definition.Location != "" {
		item.Location = &Location{Path: definition.Location}
	}
	return item
}

func cloneDefinition(definition Definition) Definition {
	definition.Actions = append([]Action(nil), definition.Actions...)
	return definition
}
