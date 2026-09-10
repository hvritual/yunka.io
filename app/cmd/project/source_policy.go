package project

import (
	"encoding/json"

	"yunka.io/app/cmd/sourceaudit"
)

const SourcePolicyRelativePath = ".yunka/source-policy.json"

// defaultSourcePolicyBytes returns the deterministic baseline source policy for a
// newly initialized Go project. It is intentionally broad: it establishes full
// inventory/build completeness without pretending to infer business layers. Teams
// may refine components/profiles explicitly after initialization.
func defaultSourcePolicyBytes() ([]byte, error) {
	policy := sourceaudit.Policy{
		SchemaVersion:      sourceaudit.SchemaVersion,
		Profiles:           []sourceaudit.Profile{{Name: "linux", GOOS: "linux", GOARCH: "amd64", CGO: false, Tags: []string{}}},
		Modules:            []sourceaudit.ModulePolicy{{Path: ".", Workspace: "off", Profiles: []string{"linux"}}},
		Components:         []sourceaudit.Component{{Name: "project", Path: ".", Kind: "production", Allow: []string{}, AllowExternal: true}},
		Exclusions:         []sourceaudit.Exclusion{},
		TestSupportImports: []string{},
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	contents, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(contents, '\n'), nil
}
