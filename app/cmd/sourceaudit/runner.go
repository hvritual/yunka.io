package sourceaudit

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type runner func(context.Context, string, string, []string, ...string) ([]byte, error)

// Only explicit cache/temp/OS inputs are inherited. Go's executable is resolved
// absolutely, never via a parent PATH or a repository-supplied package driver.
func environment(profile Profile, workspace string) []string {
	vars := map[string]string{}
	for _, key := range []string{"HOME", "USERPROFILE", "SystemRoot", "SYSTEMROOT", "WINDIR", "TMP", "TEMP", "TMPDIR", "GOPATH", "GOMODCACHE", "GOCACHE"} {
		if value := os.Getenv(key); value != "" {
			vars[key] = value
		}
	}
	for k, v := range map[string]string{"PATH": filepath.Join(runtime.GOROOT(), "bin") + string(os.PathListSeparator) + os.Getenv("PATH"), "GOROOT": runtime.GOROOT(), "GOENV": "off", "GOFLAGS": "", "GOWORK": workspace, "GOTOOLCHAIN": "local", "GO111MODULE": "on", "GOPACKAGESDRIVER": "off", "GOPROXY": "off", "GOSUMDB": "off", "GOVCS": "*:off", "GOCACHEPROG": "", "GOTELEMETRY": "off", "GOOS": profile.GOOS, "GOARCH": profile.GOARCH, "CGO_ENABLED": "0"} {
		vars[k] = v
	}
	if profile.CGO {
		vars["CGO_ENABLED"] = "1"
	}
	// Compiler feature levels must not leak from a parent environment.
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(vars))
	for _, k := range keys {
		out = append(out, k+"="+vars[k])
	}
	return out
}

type boundedBuffer struct {
	b    bytes.Buffer
	left int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.left {
		return 0, fmt.Errorf("Go output exceeds the analysis budget")
	}
	n, e := b.b.Write(p)
	b.left -= n
	return n, e
}

func runGo(ctx context.Context, goBinary, dir string, env []string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, goBinary, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.WaitDelay = time.Second
	if err := prepareCommand(cmd); err != nil {
		return nil, err
	}
	defer cleanupCommand(cmd)
	out, stderr := &boundedBuffer{left: 64 << 20}, &boundedBuffer{left: 1 << 20}
	cmd.Stdout = out
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return out.b.Bytes(), fmt.Errorf("Go command failed or exceeded its budget")
	}
	return out.b.Bytes(), nil
}

func toolchain(ctx context.Context, run runner, dir string) (string, string, map[string]bool, error) {
	exe := "go"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	binary, err := filepath.Abs(filepath.Join(runtime.GOROOT(), "bin", exe))
	if err != nil {
		return "", "", nil, err
	}
	info, err := os.Stat(binary)
	if err != nil || !info.Mode().IsRegular() {
		return "", "", nil, fmt.Errorf("checker GOROOT must contain a regular Go executable")
	}
	env := environment(Profile{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}, "off")
	b, err := run(ctx, binary, dir, env, "version")
	if err != nil {
		return "", "", nil, err
	}
	fields := strings.Fields(string(b))
	if len(fields) != 4 || fields[0] != "go" || fields[1] != "version" || fields[2] != runtime.Version() {
		return "", "", nil, fmt.Errorf("installed Go and checker build versions differ")
	}
	b, err = run(ctx, binary, dir, env, "tool", "dist", "list")
	if err != nil {
		return "", "", nil, err
	}
	targets := map[string]bool{}
	for _, s := range strings.Fields(string(b)) {
		targets[s] = true
	}
	if len(targets) == 0 {
		return "", "", nil, fmt.Errorf("Go supplied no supported targets")
	}
	return binary, fields[2], targets, nil
}

func decodePackages(data []byte) ([]packageMeta, error) {
	d := jsonDecoder(data)
	var out []packageMeta
	for {
		var p packageMeta
		err := d.Decode(&p)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid Go package metadata")
		}
		if p.ImportPath == "" {
			return nil, fmt.Errorf("Go package metadata lacks identity")
		}
		out = append(out, p)
		if len(out) > 100000 {
			return nil, fmt.Errorf("Go package metadata exceeds package budget")
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("Go returned empty package metadata")
	}
	return out, nil
}
