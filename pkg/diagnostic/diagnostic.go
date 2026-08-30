package diagnostic

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const SchemaVersion = 1

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

type ActionKind string

const (
	ActionCommand ActionKind = "command"
	ActionEdit    ActionKind = "edit"
	ActionDocs    ActionKind = "docs"
)

type Location struct {
	Path   string `json:"path,omitempty"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
	Ref    string `json:"ref,omitempty"`
}

type Action struct {
	Kind  ActionKind `json:"kind"`
	Label string     `json:"label,omitempty"`
	Value string     `json:"value"`
}

type Diagnostic struct {
	Code     string     `json:"code"`
	Severity Severity   `json:"severity"`
	Stage    string     `json:"stage"`
	Summary  string     `json:"summary"`
	Detail   string     `json:"detail,omitempty"`
	Location *Location  `json:"location,omitempty"`
	Actions  []Action   `json:"actions,omitempty"`
}

type Envelope struct {
	SchemaVersion int          `json:"schemaVersion"`
	Command       string       `json:"command"`
	OK            bool         `json:"ok"`
	Diagnostics   []Diagnostic `json:"diagnostics,omitempty"`
}

var codePattern = regexp.MustCompile(`^YUNKA-DX-[A-Z][A-Z0-9_-]*-[0-9]{3}$`)
var windowsAbsolutePattern = regexp.MustCompile(`^[A-Za-z]:[\\/]`)

func NewEnvelope(command string, diagnostics []Diagnostic) (Envelope, error) {
	normalized, err := Normalize(diagnostics)
	if err != nil {
		return Envelope{}, err
	}
	envelope := Envelope{
		SchemaVersion: SchemaVersion,
		Command:       strings.TrimSpace(command),
		OK:            true,
		Diagnostics:   normalized,
	}
	if envelope.Command == "" {
		return Envelope{}, errors.New("diagnostic: command is required")
	}
	for _, item := range normalized {
		if item.Severity == SeverityError {
			envelope.OK = false
			break
		}
	}
	return envelope, nil
}

func Normalize(diagnostics []Diagnostic) ([]Diagnostic, error) {
	result := make([]Diagnostic, len(diagnostics))
	copy(result, diagnostics)
	for index := range result {
		if err := normalizeDiagnostic(&result[index]); err != nil {
			return nil, fmt.Errorf("diagnostic[%d]: %w", index, err)
		}
	}
	sort.SliceStable(result, func(left, right int) bool {
		l, r := result[left], result[right]
		if severityRank(l.Severity) != severityRank(r.Severity) {
			return severityRank(l.Severity) < severityRank(r.Severity)
		}
		if l.Stage != r.Stage {
			return l.Stage < r.Stage
		}
		if l.Code != r.Code {
			return l.Code < r.Code
		}
		if locationKey(l.Location) != locationKey(r.Location) {
			return locationKey(l.Location) < locationKey(r.Location)
		}
		return l.Summary < r.Summary
	})
	return result, nil
}

func RenderJSON(command string, diagnostics []Diagnostic) ([]byte, error) {
	envelope, err := NewEnvelope(command, diagnostics)
	if err != nil {
		return nil, err
	}
	contents, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(contents, '\n'), nil
}

func RenderText(diagnostics []Diagnostic) (string, error) {
	normalized, err := Normalize(diagnostics)
	if err != nil {
		return "", err
	}
	var builder strings.Builder
	for index, item := range normalized {
		if index > 0 {
			builder.WriteByte('\n')
		}
		fmt.Fprintf(&builder, "%s %s  %s\n", strings.ToUpper(string(item.Severity)), item.Code, item.Summary)
		fmt.Fprintf(&builder, "  stage:    %s\n", item.Stage)
		if item.Location != nil {
			if value := formatLocation(*item.Location); value != "" {
				fmt.Fprintf(&builder, "  location: %s\n", value)
			}
		}
		if item.Detail != "" {
			fmt.Fprintf(&builder, "  detail:   %s\n", item.Detail)
		}
		for _, action := range item.Actions {
			label := strings.TrimSpace(action.Label)
			if label == "" {
				label = string(action.Kind)
			}
			fmt.Fprintf(&builder, "  action:   %s: %s\n", label, action.Value)
		}
	}
	return builder.String(), nil
}

func normalizeDiagnostic(item *Diagnostic) error {
	item.Code = strings.TrimSpace(item.Code)
	item.Stage = strings.TrimSpace(item.Stage)
	item.Summary = strings.TrimSpace(item.Summary)
	item.Detail = strings.TrimSpace(item.Detail)
	if !codePattern.MatchString(item.Code) {
		return fmt.Errorf("code %q must match %s", item.Code, codePattern)
	}
	switch item.Severity {
	case SeverityError, SeverityWarning, SeverityInfo:
	default:
		return fmt.Errorf("severity %q is unsupported", item.Severity)
	}
	if item.Stage == "" {
		return errors.New("stage is required")
	}
	if item.Summary == "" {
		return errors.New("summary is required")
	}
	if item.Location != nil {
		item.Location.Path = strings.TrimSpace(strings.ReplaceAll(item.Location.Path, "\\", "/"))
		item.Location.Ref = strings.TrimSpace(item.Location.Ref)
		if item.Location.Line < 0 || item.Location.Column < 0 {
			return errors.New("location line/column cannot be negative")
		}
		if item.Location.Path != "" && absoluteLike(item.Location.Path) {
			return fmt.Errorf("location path %q must be project-relative", item.Location.Path)
		}
	}
	for index := range item.Actions {
		action := &item.Actions[index]
		action.Label = strings.TrimSpace(action.Label)
		action.Value = strings.TrimSpace(action.Value)
		switch action.Kind {
		case ActionCommand, ActionEdit, ActionDocs:
		default:
			return fmt.Errorf("action kind %q is unsupported", action.Kind)
		}
		if action.Value == "" {
			return fmt.Errorf("action[%d] value is required", index)
		}
	}
	return nil
}

func severityRank(value Severity) int {
	switch value {
	case SeverityError:
		return 0
	case SeverityWarning:
		return 1
	default:
		return 2
	}
}

func locationKey(location *Location) string {
	if location == nil {
		return ""
	}
	return fmt.Sprintf("%s:%09d:%09d:%s", location.Path, location.Line, location.Column, location.Ref)
}

func formatLocation(location Location) string {
	value := location.Path
	if value == "" {
		value = location.Ref
	}
	if value == "" {
		return ""
	}
	if location.Line > 0 {
		value += fmt.Sprintf(":%d", location.Line)
		if location.Column > 0 {
			value += fmt.Sprintf(":%d", location.Column)
		}
	}
	return value
}

func absoluteLike(value string) bool {
	return strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") || windowsAbsolutePattern.MatchString(value) || filepath.IsAbs(filepath.FromSlash(value))
}
