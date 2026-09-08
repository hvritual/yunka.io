package applicationboundary

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func loaderGoPath() string {
	name := "go"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(runtime.GOROOT(), "bin", name)
}

// x/tools resolves exec.Command("go") against the parent process PATH before
// installing Config.Env. Do not mutate process-global PATH or execute a wrapper
// found there: require it to resolve to the canonical GOROOT binary, then query
// that absolute binary's version. A mismatch is incomplete, not a clean audit.
// The filesystem/environment is not locked against concurrent external mutation.
func verifyLoaderGo(ctx context.Context) error {
	if ctx == nil || ctx.Err() != nil {
		return fmt.Errorf("Go toolchain verification was cancelled")
	}
	expected := loaderGoPath()
	resolved, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("cannot resolve canonical Go executable from process PATH")
	}
	want, wantErr := os.Stat(expected)
	got, gotErr := os.Stat(resolved)
	if wantErr != nil || gotErr != nil || !want.Mode().IsRegular() || !got.Mode().IsRegular() || !os.SameFile(want, got) {
		return fmt.Errorf("process PATH does not resolve Go to runtime.GOROOT()/bin; put the checker toolchain first on PATH")
	}
	cmd := exec.CommandContext(ctx, expected, "version")
	cmd.Env = loadEnvironment()
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) != "go version "+runtime.Version()+" "+runtime.GOOS+"/"+runtime.GOARCH {
		return fmt.Errorf("resolved Go executable does not match the running checker toolchain")
	}
	return nil
}
