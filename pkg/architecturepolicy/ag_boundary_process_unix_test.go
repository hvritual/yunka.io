//go:build unix

package architecturepolicy

import (
	"errors"
	"os"
	"os/exec"
	"sync"
	"syscall"
)

// Each trusted compiler/probe command owns a fresh process group. Kill the group
// both on cancellation and after Wait, including a parent that exits while a
// descendant retains its output pipes. WaitDelay alone does not kill descendants.
// This does not contain a hostile process that deliberately changes its group.
func agBoundaryOwnProcessTree(cmd *exec.Cmd) (func() error, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	killGroup := func() error {
		if cmd.Process == nil {
			return nil // Start failed; there is no owned group.
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.Cancel = killGroup
	var once sync.Once
	var result error
	return func() error {
		once.Do(func() {
			result = killGroup()
			if errors.Is(result, os.ErrProcessDone) {
				result = nil
			}
		})
		return result
	}, nil
}
