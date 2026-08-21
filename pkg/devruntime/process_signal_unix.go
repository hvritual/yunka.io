//go:build !windows

package devruntime

import (
	"os"
	"syscall"
)

func signalProcess(process *os.Process) error {
	if process == nil {
		return nil
	}
	return process.Signal(syscall.SIGTERM)
}
