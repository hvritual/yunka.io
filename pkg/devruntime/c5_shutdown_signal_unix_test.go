//go:build !windows

package devruntime

import (
	"os"
	"syscall"
)

func c5ShutdownSignal() os.Signal {
	return syscall.SIGTERM
}
