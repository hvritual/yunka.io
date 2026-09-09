//go:build !unix

package sourceaudit

import (
	"fmt"
	"os/exec"
)

func prepareCommand(*exec.Cmd) error {
	return fmt.Errorf("source audit native process cleanup is unqualified on this host; use a supported Unix host")
}
func cleanupCommand(*exec.Cmd) {}
