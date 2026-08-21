package devruntime

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	applicationgraph "yunka.io/pkg/applicationgraph"
)

const (
	RuntimeReportSchemaVersion = 1
	maxRuntimeErrorBytes       = 512
)

type RuntimeRunState string

const (
	RuntimeRunStarting RuntimeRunState = "starting"
	RuntimeRunRunning  RuntimeRunState = "running"
	RuntimeRunStopping RuntimeRunState = "stopping"
	RuntimeRunStopped  RuntimeRunState = "stopped"
	RuntimeRunFailed   RuntimeRunState = "failed"
)

type ProcessState string

const (
	ProcessPending  ProcessState = "pending"
	ProcessStarting ProcessState = "starting"
	ProcessRunning  ProcessState = "running"
	ProcessReady    ProcessState = "ready"
	ProcessStopping ProcessState = "stopping"
	ProcessStopped  ProcessState = "stopped"
	ProcessFailed   ProcessState = "failed"
)

type RuntimeCoreSummary struct {
	State               string `json:"state,omitempty"`
	HealthState         string `json:"healthState,omitempty"`
	Live                bool   `json:"live"`
	Ready               bool   `json:"ready"`
	RouteCount          int    `json:"routeCount"`
	RPCClientConfigured bool   `json:"rpcClientConfigured"`
	RPCServerCount      int    `json:"rpcServerCount"`
	EventBusConfigured  bool   `json:"eventBusConfigured"`
}

type ProcessRuntimeReport struct {
	Name        string              `json:"name"`
	GraphNode   string              `json:"graphNode,omitempty"`
	State       ProcessState        `json:"state"`
	Ready       bool                `json:"ready"`
	Diagnostics *RuntimeCoreSummary `json:"diagnostics,omitempty"`
	StartedAt   string              `json:"startedAt,omitempty"`
	ReadyAt     string              `json:"readyAt,omitempty"`
	StoppedAt   string              `json:"stoppedAt,omitempty"`
	Error       string              `json:"error,omitempty"`
}

type RuntimeReport struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Application   string                 `json:"application"`
	State         RuntimeRunState        `json:"state"`
	Reason        string                 `json:"reason,omitempty"`
	Plan          []string               `json:"plan"`
	Processes     []ProcessRuntimeReport `json:"processes"`
	StartedAt     string                 `json:"startedAt"`
	UpdatedAt     string                 `json:"updatedAt"`
	FinishedAt    string                 `json:"finishedAt,omitempty"`
}

type runtimeRecorder struct {
	root      string
	statePath string
	graphPath string
	plan      Plan
	report    RuntimeReport
	now       func() time.Time
}

func newRuntimeRecorder(root string, plan Plan, now func() time.Time) (*runtimeRecorder, error) {
	if plan.Runtime == nil {
		return nil, nil
	}
	if now == nil {
		now = time.Now
	}
	statePath, err := resolveArtifactPath(root, plan.Runtime.StatePath)
	if err != nil {
		return nil, fmt.Errorf("devruntime: runtime state path: %w", err)
	}
	graphPath, err := resolveArtifactPath(root, plan.Runtime.GraphPath)
	if err != nil {
		return nil, fmt.Errorf("devruntime: runtime graph path: %w", err)
	}
	if statePath == graphPath {
		return nil, errors.New("devruntime: runtime state and graph paths must differ")
	}
	stamp := formatRuntimeTime(now())
	report := RuntimeReport{
		SchemaVersion: RuntimeReportSchemaVersion,
		Application:   strings.TrimSpace(plan.Runtime.Application),
		State:         RuntimeRunStarting,
		Plan:          plan.Names(),
		StartedAt:     stamp,
		UpdatedAt:     stamp,
	}
	for _, process := range plan.Processes {
		report.Processes = append(report.Processes, ProcessRuntimeReport{
			Name: process.Name, GraphNode: process.GraphNode, State: ProcessPending,
		})
	}
	recorder := &runtimeRecorder{root: root, statePath: statePath, graphPath: graphPath, plan: plan, report: report, now: now}
	if err := recorder.persist(); err != nil {
		return nil, err
	}
	return recorder, nil
}

func (recorder *runtimeRecorder) transition(name string, state ProcessState, summary *RuntimeCoreSummary, cause error) error {
	if recorder == nil {
		return nil
	}
	index := recorder.processIndex(name)
	if index < 0 {
		return fmt.Errorf("devruntime: runtime report process %q not found", name)
	}
	stamp := formatRuntimeTime(recorder.now())
	current := &recorder.report.Processes[index]
	current.State = state
	switch state {
	case ProcessStarting:
		if current.StartedAt == "" {
			current.StartedAt = stamp
		}
	case ProcessReady:
		current.Ready = true
		current.ReadyAt = stamp
	case ProcessStopped, ProcessFailed:
		current.StoppedAt = stamp
	}
	if summary != nil {
		copy := *summary
		current.Diagnostics = &copy
		current.Ready = copy.Ready || state == ProcessReady
	}
	if cause != nil {
		current.Error = sanitizeRuntimeError(cause.Error())
	}
	recorder.report.UpdatedAt = stamp
	return recorder.persist()
}

func (recorder *runtimeRecorder) setState(state RuntimeRunState, reason string, finished bool) error {
	if recorder == nil {
		return nil
	}
	stamp := formatRuntimeTime(recorder.now())
	recorder.report.State = state
	recorder.report.Reason = sanitizeRuntimeError(reason)
	recorder.report.UpdatedAt = stamp
	if finished {
		recorder.report.FinishedAt = stamp
	}
	return recorder.persist()
}

func (recorder *runtimeRecorder) processState(name string) ProcessState {
	if recorder == nil {
		return ""
	}
	if index := recorder.processIndex(name); index >= 0 {
		return recorder.report.Processes[index].State
	}
	return ""
}

func (recorder *runtimeRecorder) processIndex(name string) int {
	for index := range recorder.report.Processes {
		if recorder.report.Processes[index].Name == name {
			return index
		}
	}
	return -1
}

func (recorder *runtimeRecorder) persist() error {
	if recorder == nil {
		return nil
	}
	if err := writeRuntimeReportAbsolute(recorder.statePath, recorder.report); err != nil {
		return err
	}
	graph, err := BuildRuntimeGraph(recorder.plan, recorder.report)
	if err != nil {
		return err
	}
	return writeGraphAbsolute(recorder.graphPath, graph)
}

func WriteRuntimeReport(root, path string, report RuntimeReport) error {
	resolved, err := resolveArtifactPath(root, path)
	if err != nil {
		return err
	}
	return writeRuntimeReportAbsolute(resolved, report)
}

func writeRuntimeReportAbsolute(path string, report RuntimeReport) error {
	report = sanitizedRuntimeReport(report)
	return atomicWrite(path, 0o600, func(writer io.Writer) error {
		encoder := json.NewEncoder(writer)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	})
}

func sanitizedRuntimeReport(report RuntimeReport) RuntimeReport {
	report.Reason = sanitizeRuntimeError(report.Reason)
	report.Plan = append([]string(nil), report.Plan...)
	report.Processes = append([]ProcessRuntimeReport(nil), report.Processes...)
	for index := range report.Processes {
		report.Processes[index].Error = sanitizeRuntimeError(report.Processes[index].Error)
		if report.Processes[index].Diagnostics != nil {
			copy := *report.Processes[index].Diagnostics
			report.Processes[index].Diagnostics = &copy
		}
	}
	return report
}

func LoadRuntimeReport(root, path string) (RuntimeReport, error) {
	resolved, err := resolveArtifactPath(root, path)
	if err != nil {
		return RuntimeReport{}, err
	}
	file, err := os.Open(resolved)
	if err != nil {
		return RuntimeReport{}, err
	}
	defer file.Close()
	var report RuntimeReport
	if err := json.NewDecoder(file).Decode(&report); err != nil {
		return RuntimeReport{}, err
	}
	if report.SchemaVersion != RuntimeReportSchemaVersion {
		return RuntimeReport{}, fmt.Errorf("devruntime: unsupported runtime report schema version %d", report.SchemaVersion)
	}
	return report, nil
}

func FormatRuntimeReport(writer io.Writer, report RuntimeReport) error {
	if writer == nil {
		return errors.New("devruntime: status writer is required")
	}
	if _, err := fmt.Fprintf(writer, "runtime application=%s state=%s updated=%s\n", report.Application, report.State, report.UpdatedAt); err != nil {
		return err
	}
	if report.Reason != "" {
		if _, err := fmt.Fprintf(writer, "reason: %s\n", report.Reason); err != nil {
			return err
		}
	}
	for index, process := range report.Processes {
		if _, err := fmt.Fprintf(writer, "%02d %-24s %-9s ready=%t", index+1, process.Name, process.State, process.Ready); err != nil {
			return err
		}
		if process.GraphNode != "" {
			if _, err := fmt.Fprintf(writer, " graph=%s", process.GraphNode); err != nil {
				return err
			}
		}
		if process.Error != "" {
			if _, err := fmt.Fprintf(writer, " error=%s", process.Error); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(writer); err != nil {
			return err
		}
	}
	return nil
}

func writeGraphAbsolute(path string, graph applicationgraph.Graph) error {
	return atomicWrite(path, 0o600, func(writer io.Writer) error {
		return applicationgraph.Encode(writer, graph)
	})
}

func atomicWrite(path string, mode os.FileMode, encode func(io.Writer) error) error {
	if encode == nil {
		return errors.New("devruntime: atomic writer encoder is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".yunka-runtime-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	remove := true
	defer func() {
		_ = temporary.Close()
		if remove {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		return err
	}
	if err := encode(temporary); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	remove = false
	return os.Chmod(path, mode)
}

func resolveArtifactPath(root, value string) (string, error) {
	root, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil {
		return "", err
	}
	if root == "" {
		return "", errors.New("project root is required")
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("artifact path is required")
	}
	if filepath.IsAbs(value) {
		return "", errors.New("artifact path must be relative to project root")
	}
	candidate := filepath.Clean(filepath.Join(root, value))
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("artifact path escapes project root")
	}
	rootReal := root
	if resolved, resolveErr := filepath.EvalSymlinks(root); resolveErr == nil {
		rootReal = resolved
	}
	if info, statErr := os.Lstat(candidate); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("artifact path must not be a symbolic link")
		}
		if resolved, resolveErr := filepath.EvalSymlinks(candidate); resolveErr == nil {
			if err := ensureInside(rootReal, resolved); err != nil {
				return "", err
			}
		}
	} else if !os.IsNotExist(statErr) {
		return "", statErr
	}
	ancestor := filepath.Dir(candidate)
	for {
		if _, statErr := os.Lstat(ancestor); statErr == nil {
			resolved, resolveErr := filepath.EvalSymlinks(ancestor)
			if resolveErr != nil {
				return "", resolveErr
			}
			if err := ensureInside(rootReal, resolved); err != nil {
				return "", err
			}
			break
		} else if !os.IsNotExist(statErr) {
			return "", statErr
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", errors.New("artifact parent is not reachable from project root")
		}
		ancestor = parent
	}
	return candidate, nil
}

func ensureInside(root, candidate string) error {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("artifact path resolves outside project root")
	}
	return nil
}

var (
	bearerPattern = regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._~+/=-]+`)
	secretPattern = regexp.MustCompile(`(?i)(authorization|token|secret|password|passwd|api[_-]?key)\s*[:=]\s*[^\s,;]+`)
)

func sanitizeRuntimeError(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = bearerPattern.ReplaceAllString(value, "Bearer <redacted>")
	value = secretPattern.ReplaceAllStringFunc(value, func(match string) string {
		separator := strings.IndexAny(match, ":=")
		if separator < 0 {
			return "<redacted>"
		}
		return strings.TrimSpace(match[:separator]) + "=<redacted>"
	})
	if len(value) <= maxRuntimeErrorBytes {
		return value
	}
	value = value[:maxRuntimeErrorBytes]
	for !utf8.ValidString(value) && len(value) > 0 {
		value = value[:len(value)-1]
	}
	return strings.TrimSpace(value)
}

func formatRuntimeTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}
