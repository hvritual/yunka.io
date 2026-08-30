package dev

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"yunka.io/pkg/devruntime"
)

const defaultRuntimeEvidencePollInterval = 25 * time.Millisecond

type devRunFunc func(context.Context, devruntime.Plan, devruntime.RunOptions) error
type runtimeReportLoadFunc func(string, string) (devruntime.RuntimeReport, error)

type runtimeEvidenceState struct {
	processStates map[string]devruntime.ProcessState
	runtimeState  devruntime.RuntimeRunState
}

func runWithEvidence(ctx context.Context, plan devruntime.Plan, root string, evidence, stdout, stderr io.Writer) error {
	return runWithEvidenceOptions(ctx, plan, root, evidence, stdout, stderr, devruntime.Run, devruntime.LoadRuntimeReport, defaultRuntimeEvidencePollInterval)
}

func runWithEvidenceOptions(
	ctx context.Context,
	plan devruntime.Plan,
	root string,
	evidence io.Writer,
	stdout io.Writer,
	stderr io.Writer,
	run devRunFunc,
	load runtimeReportLoadFunc,
	pollInterval time.Duration,
) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if evidence == nil {
		evidence = io.Discard
	}
	if pollInterval <= 0 {
		pollInterval = defaultRuntimeEvidencePollInterval
	}
	if run == nil {
		run = devruntime.Run
	}
	if load == nil {
		load = devruntime.LoadRuntimeReport
	}

	renderDevPlan(evidence, plan)
	if plan.Runtime == nil {
		fmt.Fprintln(evidence, "DEV evidence runtime-report=disabled")
		err := run(ctx, plan, devruntime.RunOptions{Root: root, Stdout: stdout, Stderr: stderr})
		if err != nil {
			fmt.Fprintln(evidence, "DEV FAILED runtime")
			fmt.Fprintln(evidence, "DEV next: yunka doctor")
		}
		return err
	}

	statePath := filepath.ToSlash(strings.TrimSpace(plan.Runtime.StatePath))
	graphPath := filepath.ToSlash(strings.TrimSpace(plan.Runtime.GraphPath))
	fmt.Fprintf(evidence, "DEV evidence state=%s graph=%s\n", statePath, graphPath)

	baselineStartedAt := ""
	if baseline, err := load(root, statePath); err == nil {
		baselineStartedAt = strings.TrimSpace(baseline.StartedAt)
	}
	freshReport := func(report devruntime.RuntimeReport) bool {
		startedAt := strings.TrimSpace(report.StartedAt)
		return startedAt != "" && startedAt != baselineStartedAt
	}

	result := make(chan error, 1)
	go func() {
		result <- run(ctx, plan, devruntime.RunOptions{Root: root, Stdout: stdout, Stderr: stderr})
	}()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	observed := runtimeEvidenceState{processStates: map[string]devruntime.ProcessState{}}
	for {
		select {
		case runErr := <-result:
			report, loadErr := load(root, statePath)
			fresh := loadErr == nil && freshReport(report)
			if fresh {
				renderRuntimeEvidence(evidence, report, &observed)
			}
			if runErr != nil {
				if observed.runtimeState != devruntime.RuntimeRunFailed {
					fmt.Fprintln(evidence, "DEV FAILED runtime")
				}
				if fresh {
					fmt.Fprintf(evidence, "DEV inspect: %s\n", runtimeStatusCommand(plan, statePath))
				}
				fmt.Fprintln(evidence, "DEV next: yunka doctor")
			}
			return runErr
		case <-ticker.C:
			report, err := load(root, statePath)
			if err != nil || !freshReport(report) {
				continue
			}
			renderRuntimeEvidence(evidence, report, &observed)
		}
	}
}

func renderDevPlan(writer io.Writer, plan devruntime.Plan) {
	names := plan.Names()
	fmt.Fprintf(writer, "DEV plan processes=%d names=%s\n", len(names), strings.Join(names, ","))
}

func renderRuntimeEvidence(writer io.Writer, report devruntime.RuntimeReport, observed *runtimeEvidenceState) {
	if observed == nil {
		return
	}
	if observed.processStates == nil {
		observed.processStates = map[string]devruntime.ProcessState{}
	}
	for _, process := range report.Processes {
		previous := observed.processStates[process.Name]
		if previous == process.State {
			continue
		}
		observed.processStates[process.Name] = process.State
		switch process.State {
		case devruntime.ProcessStarting:
			fmt.Fprintf(writer, "DEV STARTING process=%s\n", process.Name)
		case devruntime.ProcessRunning:
			fmt.Fprintf(writer, "DEV RUNNING process=%s\n", process.Name)
		case devruntime.ProcessReady:
			fmt.Fprintf(writer, "DEV READY process=%s\n", process.Name)
		case devruntime.ProcessFailed:
			fmt.Fprintf(writer, "DEV FAILED process=%s\n", process.Name)
		}
	}
	if observed.runtimeState == report.State {
		return
	}
	observed.runtimeState = report.State
	application := strings.TrimSpace(report.Application)
	if application == "" {
		application = devruntime.DefaultRuntimeApplication
	}
	switch report.State {
	case devruntime.RuntimeRunRunning:
		fmt.Fprintf(writer, "DEV READY application=%s\n", application)
	case devruntime.RuntimeRunFailed:
		fmt.Fprintf(writer, "DEV FAILED application=%s\n", application)
	case devruntime.RuntimeRunStopped:
		fmt.Fprintf(writer, "DEV STOPPED application=%s\n", application)
	}
}

func runtimeStatusCommand(plan devruntime.Plan, statePath string) string {
	parts := []string{"yunka", "dev", "status"}
	if plan.Closure {
		parts = append(parts, "--closure")
	}
	if strings.TrimSpace(statePath) != "" {
		parts = append(parts, "--state", filepath.ToSlash(strings.TrimSpace(statePath)))
	}
	return strings.Join(parts, " ")
}
