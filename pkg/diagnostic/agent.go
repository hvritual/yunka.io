package diagnostic

import (
	"encoding/json"
	"errors"
	"strings"
)

const AgentSchemaVersion = 1

type AgentCause struct {
	Summary string `json:"summary"`
	Detail  string `json:"detail,omitempty"`
}

type AgentDiagnostic struct {
	Code        string     `json:"code"`
	Severity    Severity   `json:"severity"`
	Stage       string     `json:"stage"`
	Cause       AgentCause `json:"cause"`
	Target      *Location  `json:"target"`
	Remediation []Action   `json:"remediation"`
	Retry       *Action    `json:"retry"`
}

type AgentEnvelope struct {
	SchemaVersion int               `json:"schemaVersion"`
	Command       string            `json:"command"`
	OK            bool              `json:"ok"`
	Diagnostics   []AgentDiagnostic `json:"diagnostics"`
}

// NewAgentEnvelope projects the canonical Diagnostic presentation contract into
// an agent-oriented remediation contract. It does not discover causes or
// ownership itself: cause evidence, target location, and remediation actions
// come from the already-normalized diagnostic producer. Retry is the original
// read/check command and remains non-executing metadata.
func NewAgentEnvelope(command string, diagnostics []Diagnostic, ok bool) (AgentEnvelope, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return AgentEnvelope{}, errors.New("diagnostic: agent command is required")
	}
	normalized, err := Normalize(diagnostics)
	if err != nil {
		return AgentEnvelope{}, err
	}
	items := make([]AgentDiagnostic, 0, len(normalized))
	for _, item := range normalized {
		projected := AgentDiagnostic{
			Code:     item.Code,
			Severity: item.Severity,
			Stage:    item.Stage,
			Cause: AgentCause{
				Summary: item.Summary,
				Detail:  item.Detail,
			},
			Target:      cloneLocation(item.Location),
			Remediation: append([]Action{}, item.Actions...),
		}
		if !ok && item.Severity != SeverityInfo {
			projected.Retry = &Action{
				Kind:  ActionCommand,
				Label: "Retry after remediation",
				Value: command,
			}
		}
		items = append(items, projected)
	}
	return AgentEnvelope{
		SchemaVersion: AgentSchemaVersion,
		Command:       command,
		OK:            ok,
		Diagnostics:   items,
	}, nil
}

func RenderAgentJSON(command string, diagnostics []Diagnostic, ok bool) ([]byte, error) {
	envelope, err := NewAgentEnvelope(command, diagnostics, ok)
	if err != nil {
		return nil, err
	}
	contents, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(contents, '\n'), nil
}

func cloneLocation(location *Location) *Location {
	if location == nil {
		return nil
	}
	copy := *location
	return &copy
}
