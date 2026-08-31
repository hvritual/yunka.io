//go:build windows

package devruntime

import "os"

func c5ShutdownSignal() os.Signal {
	return os.Interrupt
}
