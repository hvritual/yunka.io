package dev

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hvritual/yunka.io/pkg/devruntime"
	"github.com/hvritual/yunka.io/pkg/diagnostic"
)

const (
	runtimeEventSchemaVersion = 1
	eventFormatText           = "text"
	eventFormatJSONL          = "jsonl"

	eventPlanResolved       = "plan.resolved"
	eventEvidenceConfigured = "runtime.evidence.configured"
	eventEvidenceDisabled   = "runtime.evidence.disabled"
	eventProcessState       = "process.state"
	eventApplicationState   = "application.state"
	eventClosureComplete    = "runtime.closure.complete"
	eventRuntimeDiagnostic  = "runtime.diagnostic"
)

type RuntimeEvent struct {
	SchemaVersion int                               `json:"schemaVersion"`
	Sequence      uint64                            `json:"sequence"`
	Type          string                            `json:"type"`
	Source        string                            `json:"source"`
	Application   string                            `json:"application,omitempty"`
	Process       string                            `json:"process,omitempty"`
	State         string                            `json:"state,omitempty"`
	Ready         *bool                             `json:"ready,omitempty"`
	Processes     []string                          `json:"processes,omitempty"`
	GraphNode     string                            `json:"graphNode,omitempty"`
	GraphNodes    []string                          `json:"graphNodes,omitempty"`
	StatePath     string                            `json:"statePath,omitempty"`
	GraphPath     string                            `json:"graphPath,omitempty"`
	Diagnostics   *devruntime.RuntimeCoreSummary    `json:"diagnostics,omitempty"`
	Diagnostic    *diagnostic.AgentDiagnostic       `json:"diagnostic,omitempty"`
	ReportUpdated string                            `json:"reportUpdatedAt,omitempty"`
}

type runtimeEventStream struct {
	writer   io.Writer
	sequence uint64
}

func newRuntimeEventStream(writer io.Writer) *runtimeEventStream {
	if writer == nil {
		writer = io.Discard
	}
	return &runtimeEventStream{writer: writer}
}

func (stream *runtimeEventStream) emit(event RuntimeEvent) error {
	if stream == nil {
		return nil
	}
	stream.sequence++
	event.SchemaVersion = runtimeEventSchemaVersion
	event.Sequence = stream.sequence
	encoder := json.NewEncoder(stream.writer)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(event)
}

func (stream *runtimeEventStream) plan(plan devruntime.Plan) error {
	return stream.emit(RuntimeEvent{
		Type:      eventPlanResolved,
		Source:    "dev-plan",
		Processes: append([]string(nil), plan.Names()...),
	})
}

func (stream *runtimeEventStream) evidence(plan devruntime.Plan) error {
	if plan.Runtime == nil {
		return stream.emit(RuntimeEvent{Type: eventEvidenceDisabled, Source: "dev-plan"})
	}
	return stream.emit(RuntimeEvent{
		Type:        eventEvidenceConfigured,
		Source:      "dev-plan",
		Application: canonicalApplication(plan.Runtime.Application),
		StatePath:   filepath.ToSlash(strings.TrimSpace(plan.Runtime.StatePath)),
		GraphPath:   filepath.ToSlash(strings.TrimSpace(plan.Runtime.GraphPath)),
	})
}

func (stream *runtimeEventStream) report(plan devruntime.Plan, report devruntime.RuntimeReport, observed *runtimeEventState) error {
	if stream == nil || observed == nil {
		return nil
	}
	if observed.processStates == nil {
		observed.processStates = map[string]devruntime.ProcessState{}
	}
	processes := append([]devruntime.ProcessRuntimeReport(nil), report.Processes...)
	sort.Slice(processes, func(i, j int) bool { return processes[i].Name < processes[j].Name })
	for _, process := range processes {
		if observed.processStates[process.Name] == process.State {
			continue
		}
		observed.processStates[process.Name] = process.State
		ready := process.Ready
		var summary *devruntime.RuntimeCoreSummary
		if process.Diagnostics != nil {
			copy := *process.Diagnostics
			summary = &copy
		}
		if err := stream.emit(RuntimeEvent{
			Type:          eventProcessState,
			Source:        "runtime-report",
			Application:   canonicalApplication(report.Application),
			Process:       process.Name,
			State:         string(process.State),
			Ready:         &ready,
			GraphNode:     process.GraphNode,
			GraphNodes:    append([]string(nil), process.GraphNodes...),
			Diagnostics:   summary,
			ReportUpdated: report.UpdatedAt,
		}); err != nil {
			return err
		}
	}
	if observed.runtimeState != report.State {
		observed.runtimeState = report.State
		if err := stream.emit(RuntimeEvent{
			Type:          eventApplicationState,
			Source:        "runtime-report",
			Application:   canonicalApplication(report.Application),
			State:         string(report.State),
			ReportUpdated: report.UpdatedAt,
		}); err != nil {
			return err
		}
	}
	if plan.Closure && !observed.closureComplete {
		if err := devruntime.ValidateRuntimeClosure(plan, report); err == nil {
			observed.closureComplete = true
			if err := stream.emit(RuntimeEvent{
				Type:          eventClosureComplete,
				Source:        "runtime-report",
				Application:   canonicalApplication(report.Application),
				State:         "complete",
				ReportUpdated: report.UpdatedAt,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (stream *runtimeEventStream) failure(plan devruntime.Plan, statePath string, reportReadable bool) error {
	item := diagnostic.MustDefinition(diagnostic.CodeRuntimeFailure).Diagnostic(diagnostic.SeverityError)
	item.Detail = "canonical local runtime supervision returned an error; the runtime stream intentionally omits raw process error details"
	if strings.TrimSpace(statePath) != "" {
		item.Location = &diagnostic.Location{Path: filepath.ToSlash(strings.TrimSpace(statePath))}
	}
	if reportReadable {
		item.Actions = append([]diagnostic.Action{{
			Kind:  diagnostic.ActionCommand,
			Label: "Inspect runtime status",
			Value: runtimeStatusCommand(plan, statePath),
		}}, item.Actions...)
	}
	envelope, err := diagnostic.NewAgentEnvelope("yunka dev", []diagnostic.Diagnostic{item}, false)
	if err != nil {
		return err
	}
	if len(envelope.Diagnostics) != 1 {
		return fmt.Errorf("dev: runtime failure diagnostic projection returned %d diagnostics", len(envelope.Diagnostics))
	}
	projected := envelope.Diagnostics[0]
	return stream.emit(RuntimeEvent{
		Type:       eventRuntimeDiagnostic,
		Source:     "dev-supervisor",
		Diagnostic: &projected,
	})
}

type runtimeEventState struct {
	processStates   map[string]devruntime.ProcessState
	runtimeState    devruntime.RuntimeRunState
	closureComplete bool
}

func renderRuntimeStatusEvents(writer io.Writer, plan devruntime.Plan, statePath string, report devruntime.RuntimeReport, closure bool) error {
	stream := newRuntimeEventStream(writer)
	if err := stream.emit(RuntimeEvent{
		Type:          "runtime.snapshot",
		Source:        "runtime-report",
		Application:   canonicalApplication(report.Application),
		State:         string(report.State),
		StatePath:     filepath.ToSlash(strings.TrimSpace(statePath)),
		Processes:     append([]string(nil), report.Plan...),
		ReportUpdated: report.UpdatedAt,
	}); err != nil {
		return err
	}
	observed := runtimeEventState{processStates: map[string]devruntime.ProcessState{}}
	if err := stream.report(plan, report, &observed); err != nil {
		return err
	}
	if closure && !observed.closureComplete {
		return fmt.Errorf("dev status: closure was requested but no complete closure event could be projected")
	}
	return nil
}

func canonicalApplication(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return devruntime.DefaultRuntimeApplication
	}
	return value
}
