//go:build windows

package devruntime

import "os"

func signalProcess(process *os.Process) error {
	if process == nil {
		return nil
	}
	return process.Signal(os.Interrupt)
}
