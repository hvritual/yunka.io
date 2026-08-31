package devruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	maxReadinessResponseBytes = 1 << 20
	maxReadinessErrorBytes    = 64 << 10
	maxSingleProbeDuration    = 5 * time.Second
	maxKillWaitDuration       = 2 * time.Second
)

type RunOptions struct {
	Root       string
	Stdout     io.Writer
	Stderr     io.Writer
	Environ    []string
	HTTPClient *http.Client
	Now        func() time.Time
}

type processExit struct {
	name string
	err  error
}

type processExitError struct {
	name string
	err  error
}

func (current *processExitError) Error() string {
	if current.err == nil {
		return fmt.Sprintf("devruntime: process %s exited unexpectedly", current.name)
	}
	return fmt.Sprintf("devruntime: process %s exited: %v", current.name, current.err)
}

func (current *processExitError) Unwrap() error { return current.err }

type supervisedProcess struct {
	process Process
	command *exec.Cmd
	exited  chan struct{}
	mu      sync.Mutex
	err     error
}

func (current *supervisedProcess) setResult(err error) {
	current.mu.Lock()
	current.err = err
	current.mu.Unlock()
	close(current.exited)
}

func (current *supervisedProcess) result() error {
	current.mu.Lock()
	defer current.mu.Unlock()
	return current.err
}

func (current *supervisedProcess) done() bool {
	select {
	case <-current.exited:
		return true
	default:
		return false
	}
}

func Run(ctx context.Context, plan Plan, options RunOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}
	root := strings.TrimSpace(options.Root)
	if root == "" {
		root = "."
	}
	if options.Stdout == nil {
		options.Stdout = os.Stdout
	}
	if options.Stderr == nil {
		options.Stderr = os.Stderr
	}
	if options.Environ == nil {
		options.Environ = os.Environ()
	}
	if options.HTTPClient == nil {
		options.HTTPClient = &http.Client{}
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if len(plan.Processes) == 0 {
		return errors.New("devruntime: empty plan")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	recorder, err := newRuntimeRecorder(root, plan, options.Now)
	if err != nil {
		return err
	}
	exits := make(chan processExit, len(plan.Processes))
	handles := make([]*supervisedProcess, 0, len(plan.Processes))
	var trigger error

	for _, current := range plan.Processes {
		process := normalizeProcess(current)
		if exit, ok := pollProcessExit(exits); ok {
			trigger = unexpectedProcessExit(exit)
			_ = markProcessExit(recorder, exit)
			break
		}
		if err := ctx.Err(); err != nil {
			break
		}
		dir, err := resolveWorkingDir(root, process.WorkingDir)
		if err != nil {
			trigger = fmt.Errorf("devruntime: process %s: %w", process.Name, err)
			break
		}
		if len(process.Command) == 0 || strings.TrimSpace(process.Command[0]) == "" {
			trigger = fmt.Errorf("devruntime: process %s has no command", process.Name)
			break
		}
		if err := recorder.transition(process.Name, ProcessStarting, nil, nil); err != nil {
			trigger = err
			break
		}

		command := exec.Command(process.Command[0], process.Command[1:]...)
		prepareProcess(command)
		command.Dir = dir
		command.Env = inheritedEnvironment(options.Environ, process.InheritEnv)
		command.Stdout = &prefixWriter{prefix: "[" + process.Name + "] ", writer: options.Stdout}
		command.Stderr = &prefixWriter{prefix: "[" + process.Name + "] ", writer: options.Stderr}
		if err := command.Start(); err != nil {
			trigger = fmt.Errorf("devruntime: start %s: %w", process.Name, err)
			_ = recorder.transition(process.Name, ProcessFailed, nil, trigger)
			break
		}
		handle := &supervisedProcess{process: process, command: command, exited: make(chan struct{})}
		handles = append(handles, handle)
		go func(handle *supervisedProcess) {
			err := handle.command.Wait()
			handle.setResult(err)
			exits <- processExit{name: handle.process.Name, err: err}
		}(handle)
		if err := recorder.transition(process.Name, ProcessRunning, nil, nil); err != nil {
			trigger = err
			break
		}

		if process.Readiness != nil {
			summary, readyErr := waitForReadinessSnapshot(ctx, process, options, exits)
			if readyErr != nil {
				if ctx.Err() == nil {
					trigger = readyErr
					failedName := process.Name
					var exitErr *processExitError
					if errors.As(readyErr, &exitErr) {
						failedName = exitErr.name
					}
					_ = recorder.transition(failedName, ProcessFailed, summary, readyErr)
				}
				break
			}
			if err := recorder.transition(process.Name, ProcessReady, summary, nil); err != nil {
				trigger = err
				break
			}
		}
	}

	if trigger == nil && ctx.Err() == nil && len(handles) == len(plan.Processes) {
		if err := recorder.setState(RuntimeRunRunning, "", false); err != nil {
			trigger = err
		} else {
			select {
			case <-ctx.Done():
			case exit := <-exits:
				trigger = unexpectedProcessExit(exit)
				_ = markProcessExit(recorder, exit)
			}
		}
	}

	if recorder != nil {
		reason := ""
		state := RuntimeRunStopping
		if trigger != nil {
			reason = trigger.Error()
		} else if ctx.Err() != nil {
			reason = ctx.Err().Error()
		}
		_ = recorder.setState(state, reason, false)
	}

	shutdownTimeout := DefaultRuntimeShutdownTimeout
	if plan.Runtime != nil {
		if configured, parseErr := runtimeShutdownDuration(*plan.Runtime); parseErr == nil {
			shutdownTimeout = configured
		} else if trigger == nil {
			trigger = parseErr
		}
	}
	shutdownErr := shutdownProcesses(handles, recorder, shutdownTimeout)

	if recorder != nil {
		if trigger != nil {
			_ = recorder.setState(RuntimeRunFailed, trigger.Error(), true)
		} else if shutdownErr != nil {
			_ = recorder.setState(RuntimeRunFailed, shutdownErr.Error(), true)
		} else {
			_ = recorder.setState(RuntimeRunStopped, "", true)
		}
	}
	if trigger != nil {
		return errors.Join(trigger, shutdownErr)
	}
	return shutdownErr
}

func markProcessExit(recorder *runtimeRecorder, exit processExit) error {
	if recorder == nil {
		return nil
	}
	return recorder.transition(exit.name, ProcessFailed, nil, unexpectedProcessExit(exit))
}

func shutdownProcesses(handles []*supervisedProcess, recorder *runtimeRecorder, timeout time.Duration) error {
	if len(handles) == 0 {
		return nil
	}
	if timeout <= 0 {
		timeout = DefaultRuntimeShutdownTimeout
	}
	deadline := time.Now().Add(timeout)
	var failures []error
	for index := len(handles) - 1; index >= 0; index-- {
		handle := handles[index]
		if handle == nil || handle.command == nil || handle.command.Process == nil {
			continue
		}
		if handle.done() {
			if recorder.processState(handle.process.Name) != ProcessFailed {
				if err := recorder.transition(handle.process.Name, ProcessStopped, nil, nil); err != nil {
					failures = append(failures, err)
				}
			}
			continue
		}
		if recorder.processState(handle.process.Name) != ProcessFailed {
			if err := recorder.transition(handle.process.Name, ProcessStopping, nil, nil); err != nil {
				failures = append(failures, err)
			}
		}
		if err := signalProcess(handle.command.Process); err != nil && !errors.Is(err, os.ErrProcessDone) {
			failures = append(failures, fmt.Errorf("devruntime: signal %s: %w", handle.process.Name, err))
		}
		remaining := time.Until(deadline)
		if remaining > 0 {
			timer := time.NewTimer(remaining)
			select {
			case <-handle.exited:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
			case <-timer.C:
			}
		}
		if !handle.done() {
			if err := killProcess(handle.command.Process); err != nil && !errors.Is(err, os.ErrProcessDone) {
				failures = append(failures, fmt.Errorf("devruntime: kill %s: %w", handle.process.Name, err))
			}
			timer := time.NewTimer(maxKillWaitDuration)
			select {
			case <-handle.exited:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
			case <-timer.C:
				failures = append(failures, fmt.Errorf("devruntime: process %s did not exit after kill", handle.process.Name))
			}
		}
		if recorder.processState(handle.process.Name) != ProcessFailed {
			if err := recorder.transition(handle.process.Name, ProcessStopped, nil, nil); err != nil {
				failures = append(failures, err)
			}
		}
	}
	return errors.Join(failures...)
}

func waitForReadiness(ctx context.Context, process Process, options RunOptions, exits <-chan processExit) error {
	_, err := waitForReadinessSnapshot(ctx, process, options, exits)
	return err
}

func waitForReadinessSnapshot(ctx context.Context, process Process, options RunOptions, exits <-chan processExit) (*RuntimeCoreSummary, error) {
	if process.Readiness == nil {
		return nil, nil
	}
	if err := validateReadiness(process.Readiness); err != nil {
		return nil, fmt.Errorf("devruntime: process %s readiness: %w", process.Name, err)
	}
	timeout, interval, err := readinessDurations(process.Readiness)
	if err != nil {
		return nil, fmt.Errorf("devruntime: process %s readiness: %w", process.Name, err)
	}
	readyCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	token := ""
	if process.Readiness.TokenEnv != "" {
		var ok bool
		token, ok = environmentValue(options.Environ, process.Readiness.TokenEnv)
		if !ok || strings.TrimSpace(token) == "" {
			return nil, fmt.Errorf("devruntime: process %s readiness token environment %s is not set", process.Name, process.Readiness.TokenEnv)
		}
	}

	var lastErr error
	var lastSummary *RuntimeCoreSummary
	for {
		if exit, ok := pollProcessExit(exits); ok {
			return lastSummary, unexpectedProcessExit(exit)
		}

		probeCtx, probeCancel := readinessProbeContext(readyCtx)
		type probeResult struct {
			ready   bool
			summary *RuntimeCoreSummary
			err     error
		}
		probeResults := make(chan probeResult, 1)
		go func() {
			ready, summary, err := probeReadinessSnapshot(probeCtx, options.HTTPClient, process.Readiness, token)
			probeResults <- probeResult{ready: ready, summary: summary, err: err}
		}()

		var current probeResult
		select {
		case current = <-probeResults:
			probeCancel()
		case exit := <-exits:
			probeCancel()
			return lastSummary, unexpectedProcessExit(exit)
		case <-readyCtx.Done():
			probeCancel()
			if ctx.Err() != nil {
				return lastSummary, ctx.Err()
			}
			if lastErr != nil {
				return lastSummary, fmt.Errorf("devruntime: process %s readiness timed out after %s: %w", process.Name, timeout, lastErr)
			}
			return lastSummary, fmt.Errorf("devruntime: process %s readiness timed out after %s", process.Name, timeout)
		}
		if current.summary != nil {
			lastSummary = current.summary
		}
		if current.ready {
			return current.summary, nil
		}
		if current.err != nil {
			lastErr = current.err
		}

		timer := time.NewTimer(interval)
		select {
		case <-readyCtx.Done():
			timer.Stop()
			if ctx.Err() != nil {
				return lastSummary, ctx.Err()
			}
			if lastErr != nil {
				return lastSummary, fmt.Errorf("devruntime: process %s readiness timed out after %s: %w", process.Name, timeout, lastErr)
			}
			return lastSummary, fmt.Errorf("devruntime: process %s readiness timed out after %s", process.Name, timeout)
		case exit := <-exits:
			timer.Stop()
			return lastSummary, unexpectedProcessExit(exit)
		case <-timer.C:
		}
	}
}

func readinessProbeContext(parent context.Context) (context.Context, context.CancelFunc) {
	deadline, ok := parent.Deadline()
	if !ok {
		return context.WithTimeout(parent, maxSingleProbeDuration)
	}
	remaining := time.Until(deadline)
	if remaining > maxSingleProbeDuration {
		remaining = maxSingleProbeDuration
	}
	if remaining <= 0 {
		remaining = time.Nanosecond
	}
	return context.WithTimeout(parent, remaining)
}

func probeReadiness(ctx context.Context, client *http.Client, readiness *Readiness, token string) (bool, error) {
	ready, _, err := probeReadinessSnapshot(ctx, client, readiness, token)
	return ready, err
}

func probeReadinessSnapshot(ctx context.Context, client *http.Client, readiness *Readiness, token string) (bool, *RuntimeCoreSummary, error) {
	if readiness == nil {
		return true, nil, nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, readiness.URL, nil)
	if err != nil {
		return false, nil, err
	}
	request.Header.Set("Accept", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	response, err := doWithoutRedirect(client, request)
	if err != nil {
		return false, nil, err
	}
	defer response.Body.Close()

	expected := readiness.ExpectedStatus
	if expected == 0 {
		expected = http.StatusOK
	}
	if response.StatusCode != expected {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxReadinessErrorBytes))
		return false, nil, fmt.Errorf("readiness endpoint returned %s, expected %d", response.Status, expected)
	}
	if !readiness.DiagnosticsReady && !readiness.CaptureDiagnostics {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxReadinessErrorBytes))
		return true, nil, nil
	}

	var report struct {
		Core struct {
			State  string `json:"state"`
			Health struct {
				State string `json:"state"`
				Live  bool   `json:"live"`
				Ready bool   `json:"ready"`
			} `json:"health"`
			Runtime struct {
				RouteCount          int  `json:"routeCount"`
				RPCClientConfigured bool `json:"rpcClientConfigured"`
				RPCServerCount      int  `json:"rpcServerCount"`
				EventBusConfigured  bool `json:"eventBusConfigured"`
			} `json:"runtime"`
		} `json:"core"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxReadinessResponseBytes))
	if err := decoder.Decode(&report); err != nil {
		return false, nil, fmt.Errorf("decode diagnostics readiness: %w", err)
	}
	summary := &RuntimeCoreSummary{
		State: report.Core.State, HealthState: report.Core.Health.State,
		Live: report.Core.Health.Live, Ready: report.Core.Health.Ready,
		RouteCount:          report.Core.Runtime.RouteCount,
		RPCClientConfigured: report.Core.Runtime.RPCClientConfigured,
		RPCServerCount:      report.Core.Runtime.RPCServerCount,
		EventBusConfigured:  report.Core.Runtime.EventBusConfigured,
	}
	if readiness.DiagnosticsReady && !summary.Ready {
		return false, summary, errors.New("diagnostics reports ready=false")
	}
	return true, summary, nil
}

func doWithoutRedirect(client *http.Client, request *http.Request) (*http.Response, error) {
	if client == nil {
		client = http.DefaultClient
	}
	copy := *client
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return copy.Do(request)
}

func pollProcessExit(exits <-chan processExit) (processExit, bool) {
	select {
	case exit := <-exits:
		return exit, true
	default:
		return processExit{}, false
	}
}

func unexpectedProcessExit(exit processExit) error {
	return &processExitError{name: exit.name, err: exit.err}
}

func inheritedEnvironment(base, names []string) []string {
	if len(names) == 0 {
		return append([]string(nil), base...)
	}
	allowed := make(map[string]struct{}, len(names))
	for _, name := range names {
		allowed[strings.TrimSpace(name)] = struct{}{}
	}
	result := make([]string, 0, len(names))
	for _, entry := range base {
		key := entry
		if index := strings.IndexByte(entry, '='); index >= 0 {
			key = entry[:index]
		}
		if _, ok := allowed[key]; ok {
			result = append(result, entry)
		}
	}
	return result
}

func environmentValue(base []string, name string) (string, bool) {
	name = strings.TrimSpace(name)
	for index := len(base) - 1; index >= 0; index-- {
		entry := base[index]
		separator := strings.IndexByte(entry, '=')
		if separator < 0 {
			continue
		}
		if entry[:separator] == name {
			return entry[separator+1:], true
		}
	}
	return "", false
}

type prefixWriter struct {
	prefix    string
	writer    io.Writer
	mu        sync.Mutex
	lineStart bool
}

func (current *prefixWriter) Write(value []byte) (int, error) {
	if current.writer == nil {
		return 0, errors.New("devruntime: nil writer")
	}
	current.mu.Lock()
	defer current.mu.Unlock()
	written := 0
	for len(value) > 0 {
		if !current.lineStart {
			if _, err := io.WriteString(current.writer, current.prefix); err != nil {
				return written, err
			}
			current.lineStart = true
		}
		index := bytes.IndexByte(value, '\n')
		if index < 0 {
			n, err := current.writer.Write(value)
			written += n
			return written, err
		}
		chunk := value[:index+1]
		n, err := current.writer.Write(chunk)
		written += n
		if err != nil {
			return written, err
		}
		current.lineStart = false
		value = value[index+1:]
	}
	return written, nil
}
