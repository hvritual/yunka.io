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
)

type RunOptions struct {
	Root       string
	Stdout     io.Writer
	Stderr     io.Writer
	Environ    []string
	HTTPClient *http.Client
}

type processExit struct {
	name string
	err  error
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
	if len(plan.Processes) == 0 {
		return errors.New("devruntime: empty plan")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	runCtx, cancel := context.WithCancel(ctx)
	exits := make(chan processExit, len(plan.Processes))
	var wg sync.WaitGroup
	defer func() {
		cancel()
		wg.Wait()
	}()

	for _, current := range plan.Processes {
		process := normalizeProcess(current)
		if exit, ok := pollProcessExit(exits); ok {
			return unexpectedProcessExit(exit)
		}
		dir, err := resolveWorkingDir(root, process.WorkingDir)
		if err != nil {
			return fmt.Errorf("devruntime: process %s: %w", process.Name, err)
		}
		if len(process.Command) == 0 || strings.TrimSpace(process.Command[0]) == "" {
			return fmt.Errorf("devruntime: process %s has no command", process.Name)
		}

		command := exec.CommandContext(runCtx, process.Command[0], process.Command[1:]...)
		command.Dir = dir
		command.Env = inheritedEnvironment(options.Environ, process.InheritEnv)
		command.Stdout = &prefixWriter{prefix: "[" + process.Name + "] ", writer: options.Stdout}
		command.Stderr = &prefixWriter{prefix: "[" + process.Name + "] ", writer: options.Stderr}
		if err := command.Start(); err != nil {
			return fmt.Errorf("devruntime: start %s: %w", process.Name, err)
		}
		wg.Add(1)
		go func(name string, command *exec.Cmd) {
			defer wg.Done()
			exits <- processExit{name: name, err: command.Wait()}
		}(process.Name, command)

		if process.Readiness != nil {
			if err := waitForReadiness(runCtx, process, options, exits); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return err
			}
		}
	}

	select {
	case <-ctx.Done():
		return nil
	case exit := <-exits:
		if ctx.Err() != nil {
			return nil
		}
		return unexpectedProcessExit(exit)
	}
}

func waitForReadiness(ctx context.Context, process Process, options RunOptions, exits <-chan processExit) error {
	if process.Readiness == nil {
		return nil
	}
	if err := validateReadiness(process.Readiness); err != nil {
		return fmt.Errorf("devruntime: process %s readiness: %w", process.Name, err)
	}
	timeout, interval, err := readinessDurations(process.Readiness)
	if err != nil {
		return fmt.Errorf("devruntime: process %s readiness: %w", process.Name, err)
	}
	readyCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	token := ""
	if process.Readiness.TokenEnv != "" {
		var ok bool
		token, ok = environmentValue(options.Environ, process.Readiness.TokenEnv)
		if !ok || strings.TrimSpace(token) == "" {
			return fmt.Errorf("devruntime: process %s readiness token environment %s is not set", process.Name, process.Readiness.TokenEnv)
		}
	}

	var lastErr error
	for {
		if exit, ok := pollProcessExit(exits); ok {
			return unexpectedProcessExit(exit)
		}

		probeCtx, probeCancel := readinessProbeContext(readyCtx)
		type probeResult struct {
			ready bool
			err   error
		}
		probeResults := make(chan probeResult, 1)
		go func() {
			ready, err := probeReadiness(probeCtx, options.HTTPClient, process.Readiness, token)
			probeResults <- probeResult{ready: ready, err: err}
		}()

		var current probeResult
		select {
		case current = <-probeResults:
			probeCancel()
		case exit := <-exits:
			probeCancel()
			return unexpectedProcessExit(exit)
		case <-readyCtx.Done():
			probeCancel()
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if lastErr != nil {
				return fmt.Errorf("devruntime: process %s readiness timed out after %s: %w", process.Name, timeout, lastErr)
			}
			return fmt.Errorf("devruntime: process %s readiness timed out after %s", process.Name, timeout)
		}
		if current.ready {
			return nil
		}
		if current.err != nil {
			lastErr = current.err
		}

		timer := time.NewTimer(interval)
		select {
		case <-readyCtx.Done():
			timer.Stop()
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if lastErr != nil {
				return fmt.Errorf("devruntime: process %s readiness timed out after %s: %w", process.Name, timeout, lastErr)
			}
			return fmt.Errorf("devruntime: process %s readiness timed out after %s", process.Name, timeout)
		case exit := <-exits:
			timer.Stop()
			return unexpectedProcessExit(exit)
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
	if readiness == nil {
		return true, nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, readiness.URL, nil)
	if err != nil {
		return false, err
	}
	request.Header.Set("Accept", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	response, err := doWithoutRedirect(client, request)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()

	expected := readiness.ExpectedStatus
	if expected == 0 {
		expected = http.StatusOK
	}
	if response.StatusCode != expected {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxReadinessErrorBytes))
		return false, fmt.Errorf("readiness endpoint returned %s, expected %d", response.Status, expected)
	}
	if !readiness.DiagnosticsReady {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxReadinessErrorBytes))
		return true, nil
	}

	var report struct {
		Core struct {
			Health struct {
				Ready bool `json:"ready"`
			} `json:"health"`
		} `json:"core"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxReadinessResponseBytes))
	if err := decoder.Decode(&report); err != nil {
		return false, fmt.Errorf("decode diagnostics readiness: %w", err)
	}
	if !report.Core.Health.Ready {
		return false, errors.New("diagnostics reports ready=false")
	}
	return true, nil
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
	if exit.err == nil {
		return fmt.Errorf("devruntime: process %s exited unexpectedly", exit.name)
	}
	return fmt.Errorf("devruntime: process %s exited: %w", exit.name, exit.err)
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
