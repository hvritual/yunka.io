//go:build !windows

package devruntime

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

const c117ProcessGroupHelperEnv = "GO_WANT_C117_PROCESS_GROUP_HELPER"

func TestRunShutdownTerminatesDescendantProcessGroup(t *testing.T) {
	root := t.TempDir()
	parentReady := filepath.Join(root, "parent-ready")
	childPID := filepath.Join(root, "child.pid")
	childSignal := filepath.Join(root, "child-signal")
	plan := c5ProcessPlan(t, root, "2s", []Process{{
		Name:      "tree",
		Command:   c117ProcessGroupHelperCommand("parent", parentReady, childPID, childSignal),
		GraphNode: "service:tree",
	}})

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- Run(ctx, plan, RunOptions{Root: root, Environ: append(os.Environ(), c117ProcessGroupHelperEnv+"=1")})
	}()
	waitForRuntimeStates(t, root, []string{"tree"}, ProcessRunning)
	waitForC5HelperFiles(t, parentReady, childPID)

	pidBytes, err := os.ReadFile(childPID)
	if err != nil {
		t.Fatal(err)
	}
	child, err := strconv.Atoi(string(pidBytes))
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

func c117ProcessGroupHelperCommand(args ...string) []string {
	return append([]string{os.Args[0], "-test.run=TestC117ProcessGroupHelper", "--"}, args...)
}

func TestC117ProcessGroupHelper(t *testing.T) {
	if os.Getenv(c117ProcessGroupHelperEnv) != "1" {
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
		child := exec.Command(os.Args[0], "-test.run=TestC117ProcessGroupHelper", "--", "child", args[2], args[3])
		child.Env = append(os.Environ(), c117ProcessGroupHelperEnv+"=1")
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
		if err := os.WriteFile(args[1], []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
			os.Exit(3)
		}
		channel := make(chan os.Signal, 1)
		signal.Notify(channel, syscall.SIGTERM)
		<-channel
		if err := os.WriteFile(args[2], []byte("signaled\n"), 0o600); err != nil {
			os.Exit(4)
		}
		os.Exit(0)
	default:
		os.Exit(2)
	}
}
