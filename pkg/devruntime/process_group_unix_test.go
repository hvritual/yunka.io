//go:build !windows

package devruntime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"testing/synctest"
	"time"
)

const processGroupHelperEnv = "GO_WANT_PROCESS_GROUP_HELPER"

func TestRunShutdownTerminatesDescendantProcessGroup(t *testing.T) {
	root := t.TempDir()
	parentReady := filepath.Join(root, "parent-ready")
	childPID := filepath.Join(root, "child.pid")
	childSignal := filepath.Join(root, "child-signal")
	plan := c5ProcessPlan(t, root, "2s", []Process{{
		Name:      "tree",
		Command:   processGroupHelperCommand("parent", parentReady, childPID, childSignal),
		GraphNode: "service:tree",
	}})

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		result <- Run(ctx, plan, RunOptions{Root: root, Environ: append(os.Environ(), processGroupHelperEnv+"=1")})
	}()
	// Readiness failures must not leave the owned process tree running.
	t.Cleanup(func() {
		cancel()
		select {
		case <-finished:
		case <-time.After(5 * time.Second):
			t.Error("runtime cleanup did not finish")
		}
	})
	waitForRuntimeStates(t, root, []string{"tree"}, ProcessRunning)
	waitForC5HelperFiles(t, parentReady)

	publication, stopPublication := context.WithTimeout(ctx, 5*time.Second)
	defer stopPublication()
	child, err := waitForDescendantPID(publication, childPID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(child, syscall.SIGKILL) })

	cancel()
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runtime did not stop")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(childSignal); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("descendant did not receive shutdown signal")
}

func processGroupHelperCommand(args ...string) []string {
	return append([]string{os.Args[0], "-test.run=^TestProcessGroupHelper$", "--"}, args...)
}

func TestProcessGroupHelper(t *testing.T) {
	if os.Getenv(processGroupHelperEnv) != "1" {
		return
	}
	separator := -1
	for index, argument := range os.Args {
		if argument == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || separator+1 >= len(os.Args) {
		os.Exit(2)
	}
	args := os.Args[separator+1:]
	switch args[0] {
	case "parent":
		if len(args) != 4 {
			os.Exit(2)
		}
		child := exec.Command(os.Args[0], "-test.run=^TestProcessGroupHelper$", "--", "child", args[2], args[3])
		child.Env = append(os.Environ(), processGroupHelperEnv+"=1")
		if err := child.Start(); err != nil {
			os.Exit(3)
		}
		if err := os.WriteFile(args[1], []byte("ready\n"), 0o600); err != nil {
			_ = child.Process.Kill()
			os.Exit(4)
		}
		_ = child.Wait()
		os.Exit(0)
	case "child":
		if len(args) != 3 {
			os.Exit(2)
		}
		channel := make(chan os.Signal, 1)
		signal.Notify(channel, syscall.SIGTERM)
		// A complete PID record also promises that SIGTERM can be received.
		// The final newline is the publication marker, not file creation.
		if err := os.WriteFile(args[1], []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600); err != nil {
			os.Exit(3)
		}
		<-channel
		if err := os.WriteFile(args[2], []byte("signaled\n"), 0o600); err != nil {
			os.Exit(4)
		}
		os.Exit(0)
	default:
		os.Exit(2)
	}
}

// waitForDescendantPID accepts only a completed record from the helper.
// File creation and even a numeric prefix can be observed before a write
// finishes. Invalid completed records fail instead of becoming signal targets.
func waitForDescendantPID(ctx context.Context, path string) (int, error) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return 0, fmt.Errorf("wait for descendant PID %s: %w", path, err)
		}
		data, err := os.ReadFile(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return 0, fmt.Errorf("read descendant PID %s: %w", path, err)
		}
		if err == nil && strings.HasSuffix(string(data), "\n") {
			record := strings.TrimSuffix(string(data), "\n")
			pid, parseErr := strconv.Atoi(record)
			if parseErr != nil || pid <= 0 || strconv.Itoa(pid) != record {
				return 0, fmt.Errorf("invalid descendant PID record %q in %s", data, path)
			}
			return pid, nil
		}
		select {
		case <-ctx.Done():
			return 0, fmt.Errorf("wait for descendant PID %s, last record %q: %w", path, data, ctx.Err())
		case <-ticker.C:
		}
	}
}

func TestWaitForDescendantPIDRequiresCompletePublication(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "child.pid")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		var pid int
		var err error
		done := make(chan struct{})
		go func() {
			defer close(done)
			pid, err = waitForDescendantPID(ctx, path)
		}()
		synctest.Wait()
		select {
		case <-done:
			t.Fatalf("returned before PID file existed: pid=%d err=%v", pid, err)
		default:
		}
		for _, incomplete := range []string{"", "12", "12345"} {
			if err := os.WriteFile(path, []byte(incomplete), 0o600); err != nil {
				t.Fatal(err)
			}
			// Fake time advances the reader; Wait makes the assertion independent
			// of OS scheduling rather than hoping a wall-clock sleep is long enough.
			time.Sleep(20 * time.Millisecond)
			synctest.Wait()
			select {
			case <-done:
				t.Fatalf("returned before PID publication completed: record=%q pid=%d err=%v", incomplete, pid, err)
			default:
			}
		}
		if err := os.WriteFile(path, []byte("12345\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		time.Sleep(20 * time.Millisecond)
		synctest.Wait()
		select {
		case <-done:
			if err != nil || pid != 12345 {
				t.Fatalf("completed PID: pid=%d err=%v", pid, err)
			}
		default:
			t.Fatal("complete PID publication was not observed")
		}
	})
}

func TestWaitForDescendantPIDRejectsInvalidRecords(t *testing.T) {
	for _, record := range []string{"\n", "0\n", "-1\n", "+12\n", " 12\n", "012\n", "bad\n", "12\n34\n"} {
		t.Run(strconv.Quote(record), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "child.pid")
				if err := os.WriteFile(path, []byte(record), 0o600); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				pid, err := waitForDescendantPID(ctx, path)
				if pid != 0 || err == nil || !strings.Contains(err.Error(), "invalid descendant PID record") {
					t.Fatalf("invalid record was not rejected: pid=%d err=%v", pid, err)
				}
			})
		})
	}
}

func TestWaitForDescendantPIDBoundsIncompletePublication(t *testing.T) {
	for _, record := range []string{"absent", "", "12345"} {
		t.Run(strconv.Quote(record), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "child.pid")
				if record != "absent" {
					if err := os.WriteFile(path, []byte(record), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				defer cancel()
				start := time.Now()
				pid, err := waitForDescendantPID(ctx, path)
				if pid != 0 || !errors.Is(err, context.DeadlineExceeded) || time.Since(start) != 50*time.Millisecond {
					t.Fatalf("unbounded or accepted incomplete PID: pid=%d err=%v elapsed=%v", pid, err, time.Since(start))
				}
			})
		})
	}
}

func TestWaitForDescendantPIDPropagatesReadErrors(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		pid, err := waitForDescendantPID(ctx, t.TempDir())
		var pathError *os.PathError
		if pid != 0 || !errors.As(err, &pathError) || errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("read error was not propagated: pid=%d err=%v", pid, err)
		}
	})
}

func TestWaitForDescendantPIDHonorsCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "child.pid")
	if err := os.WriteFile(path, []byte("12345\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pid, err := waitForDescendantPID(ctx, path)
	if pid != 0 || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled wait was not stopped: pid=%d err=%v", pid, err)
	}
}
