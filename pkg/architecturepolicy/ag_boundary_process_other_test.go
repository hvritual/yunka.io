//go:build !unix

package architecturepolicy

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Do not present direct-process termination as tree-cleanup qualification on a
// platform without the required backend. Compilation remains portable; running
// this mechanism suite is explicitly incomplete until that backend is qualified.
func agBoundaryOwnProcessTree(*exec.Cmd) (func() error, error) {
	return nil, fmt.Errorf("AG-01 INCOMPLETE: process-tree cleanup is not qualified on %s", runtime.GOOS)
}
