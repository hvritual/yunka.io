package devruntime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

type RunOptions struct {
	Root    string
	Stdout  io.Writer
	Stderr  io.Writer
	Environ []string
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

	if err := ctx.Err(); err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	exits := make(chan processExit, len(plan.Processes))
	commands := make([]*exec.Cmd, 0, len(plan.Processes))
	var wg sync.WaitGroup
	for _, process := range plan.Processes {
		dir, err := resolveWorkingDir(root, process.WorkingDir)
		if err != nil {
			return err
		}
		command := exec.CommandContext(runCtx, process.Command[0], process.Command[1:]...)
		command.Dir = dir
		command.Env = inheritedEnvironment(options.Environ, process.InheritEnv)
		command.Stdout = &prefixWriter{prefix: "[" + process.Name + "] ", writer: options.Stdout}
		command.Stderr = &prefixWriter{prefix: "[" + process.Name + "] ", writer: options.Stderr}
		if err := command.Start(); err != nil {
			cancel()
			wg.Wait()
			return fmt.Errorf("devruntime: start %s: %w", process.Name, err)
		}
		commands = append(commands, command)
		wg.Add(1)
		go func(name string, command *exec.Cmd) {
			defer wg.Done()
			exits <- processExit{name: name, err: command.Wait()}
		}(process.Name, command)
	}
	defer func() { cancel(); wg.Wait() }()

	select {
	case <-ctx.Done():
		return nil
	case exit := <-exits:
		cancel()
		if ctx.Err() != nil {
			return nil
		}
		if exit.err == nil {
			return fmt.Errorf("devruntime: process %s exited unexpectedly", exit.name)
		}
		return fmt.Errorf("devruntime: process %s exited: %w", exit.name, exit.err)
	}
}

func inheritedEnvironment(base, names []string) []string {
	if len(names) == 0 {
		return append([]string(nil), base...)
	}
	allowed := make(map[string]struct{}, len(names))
	for _, name := range names {
		allowed[name] = struct{}{}
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

type prefixWriter struct {
	prefix    string
	writer   io.Writer
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
		index := strings.IndexByte(string(value), '\n')
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
		value = value[index+1:*]
	}
	return written, nil
}
