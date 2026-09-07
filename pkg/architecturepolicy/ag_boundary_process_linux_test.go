package architecturepolicy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestAGBoundaryProcessTreeCleanup(t *testing.T) {
	// /proc can disappear before open (ENOENT) or after open (ESRCH).
	// Both mean the process is gone; permission and other I/O errors do not.
	for _, err := range []error{os.ErrNotExist, syscall.ESRCH, &os.PathError{Op: "read", Path: "/proc/probe/stat", Err: syscall.ESRCH}} {
		if !agBoundaryLinuxProcessGone(err) {
			t.Fatalf("terminated process not recognized: %v", err)
		}
	}
	for _, err := range []error{nil, syscall.EACCES, syscall.EIO} {
		if agBoundaryLinuxProcessGone(err) {
			t.Fatalf("unrelated error accepted as termination: %v", err)
		}
	}
	for _, tc := range []struct {
		name, command string
		timeout       time.Duration
		want          error
	}{
		{"timeout", "sleep 60 & echo $! > child.pid; wait", 3 * time.Second, context.DeadlineExceeded},
		{"parent_exit_with_inherited_pipe", "sleep 60 & echo $! > child.pid", 10 * time.Second, exec.ErrWaitDelay},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			env := agBoundaryEnvironment(os.Environ(), t.TempDir())
			_, code, err := agBoundaryRunWithTimeout(root, env, tc.timeout, "/bin/sh", "-c", tc.command)
			if code != -1 || !errors.Is(err, tc.want) {
				t.Fatalf("unexpected termination: code=%d err=%v want=%v", code, err, tc.want)
			}
			data, err := os.ReadFile(filepath.Join(root, "child.pid"))
			if err != nil {
				t.Fatalf("AG-01 INCOMPLETE: child did not start: %v", err)
			}
			pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil || pid <= 1 {
				t.Fatalf("invalid child PID: %q", data)
			}
			defer syscall.Kill(pid, syscall.SIGKILL) // contain the failure fixture too
			deadline := time.Now().Add(3 * time.Second)
			for {
				running, err := agBoundaryLinuxProcessRunning(pid)
				if err != nil {
					t.Fatal(err)
				}
				if !running {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("descendant %d remained running after harness returned", pid)
				}
				time.Sleep(10 * time.Millisecond)
			}
		})
	}
}

// An orphan can briefly be a zombie until the host's reaper collects it. A
// zombie has terminated and cannot keep running against the fixture/cache.
func agBoundaryLinuxProcessRunning(pid int) (bool, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if agBoundaryLinuxProcessGone(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	end := strings.LastIndexByte(string(data), ')')
	if end < 0 {
		return false, fmt.Errorf("invalid process stat for %d", pid)
	}
	fields := strings.Fields(string(data)[end+1:])
	if len(fields) == 0 {
		return false, fmt.Errorf("missing process state for %d", pid)
	}
	return fields[0] != "Z" && fields[0] != "X", nil
}

func agBoundaryLinuxProcessGone(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ESRCH)
}
