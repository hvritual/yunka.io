package devruntime

import (
	"errors"
	"fmt"
	"strings"

	applicationgraph "yunka.io/pkg/applicationgraph"
)

// ValidateRuntimeClosure verifies that a runtime report is a current, complete
// observation of the explicitly planned closure. It never infers ownership from
// process names, commands, ports, or package layout.
func ValidateRuntimeClosure(plan Plan, report RuntimeReport) error {
	if !plan.Closure {
		return errors.New("devruntime: closure validation requires a closure plan")
	}
	if plan.Runtime == nil {
		return errors.New("devruntime: closure plan is missing runtime configuration")
	}
	if report.SchemaVersion != RuntimeReportSchemaVersion {
		return fmt.Errorf("devruntime: unsupported runtime report schema version %d", report.SchemaVersion)
	}

	expectedApplication := strings.TrimSpace(plan.Runtime.Application)
	if expectedApplication == "" {
		expectedApplication = DefaultRuntimeApplication
	}
	if strings.TrimSpace(report.Application) != expectedApplication {
		return fmt.Errorf("devruntime: runtime report application %q does not match closure plan %q", report.Application, expectedApplication)
	}
	if report.State != RuntimeRunRunning {
		return fmt.Errorf("devruntime: runtime closure is not running: state=%s", report.State)
	}
	if !equalClosureStrings(report.Plan, plan.Names()) {
		return fmt.Errorf("devruntime: runtime report plan does not match current closure plan")
	}
	if len(report.Processes) != len(plan.Processes) {
		return fmt.Errorf("devruntime: runtime report process count=%d does not match closure plan=%d", len(report.Processes), len(plan.Processes))
	}

	reported := make(map[string]ProcessRuntimeReport, len(report.Processes))
	for _, process := range report.Processes {
		name := strings.TrimSpace(process.Name)
		if name == "" {
			return errors.New("devruntime: runtime report contains an unnamed process")
		}
		if _, duplicate := reported[name]; duplicate {
			return fmt.Errorf("devruntime: runtime report contains duplicate process %q", name)
		}
		reported[name] = process
	}

	nodes := make(map[string]applicationgraph.Node, len(plan.BaseGraph.Nodes))
	for _, node := range plan.BaseGraph.Nodes {
		nodes[node.ID] = node
	}
	for _, expected := range plan.Processes {
		current, ok := reported[expected.Name]
		if !ok {
			return fmt.Errorf("devruntime: runtime report is missing process %q", expected.Name)
		}
		expectedNodes := processOwnedGraphNodes(expected)
		currentNodes := reportOwnedGraphNodes(current)
		if !equalOwnedGraphNodes(currentNodes, expectedNodes) {
			return fmt.Errorf("devruntime: process %q graph ownership drift: report=%v plan=%v", expected.Name, currentNodes, expectedNodes)
		}
		if current.State == ProcessFailed || strings.TrimSpace(current.Error) != "" {
			return fmt.Errorf("devruntime: process %q is failed: state=%s error=%s", expected.Name, current.State, sanitizeRuntimeError(current.Error))
		}
		if expected.Readiness != nil {
			if current.State != ProcessReady || !current.Ready {
				return fmt.Errorf("devruntime: process %q has not satisfied readiness: state=%s ready=%t", expected.Name, current.State, current.Ready)
			}
		} else if current.State != ProcessRunning && current.State != ProcessReady {
			return fmt.Errorf("devruntime: process %q is not running: state=%s", expected.Name, current.State)
		}

		requiresDiagnostics := false
		for _, graphNode := range expectedNodes {
			node, exists := nodes[graphNode]
			if !exists {
				return fmt.Errorf("devruntime: process %q graph node %q is absent from closure graph", expected.Name, graphNode)
			}
			if node.Kind == applicationgraph.NodeApplication {
				requiresDiagnostics = true
			}
		}
		if !requiresDiagnostics {
			continue
		}
		if expected.Readiness == nil || !expected.Readiness.DiagnosticsReady || !expected.Readiness.CaptureDiagnostics {
			return fmt.Errorf("devruntime: application owner process %q does not declare the required diagnostics readiness barrier", expected.Name)
		}
		if current.Diagnostics == nil {
			return fmt.Errorf("devruntime: application owner process %q has no captured diagnostics", expected.Name)
		}
		if !current.Diagnostics.Live || !current.Diagnostics.Ready {
			return fmt.Errorf("devruntime: application owner process %q diagnostics are incomplete: live=%t ready=%t", expected.Name, current.Diagnostics.Live, current.Diagnostics.Ready)
		}
		if strings.TrimSpace(current.Diagnostics.State) == "" || strings.TrimSpace(current.Diagnostics.HealthState) == "" {
			return fmt.Errorf("devruntime: application owner process %q diagnostics are missing state evidence", expected.Name)
		}
	}
	return nil
}

func equalClosureStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if strings.TrimSpace(left[index]) != strings.TrimSpace(right[index]) {
			return false
		}
	}
	return true
}
