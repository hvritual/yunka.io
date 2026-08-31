//go:build windows

package devruntime

import (
	"os"
	"os/exec"
)

func prepareProcess(*exec.Cmd) {}

func signalProcess(process *os.Process) error {
	if process == nil {
		return nil
	}
	return process.Signal(os.Interrupt)
}

func killProcess(process *os.Process) error {
	if process == nil {
		return nil
	}
	return process.Kill()
}
