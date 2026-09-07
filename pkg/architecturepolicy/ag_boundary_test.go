package architecturepolicy

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

// These tests characterize Go encapsulation, not consumer architecture quality.
// Several successful probes intentionally demonstrate a leak. A matching probe
// is evidence about a mechanism, not a security certification or an Audit rule.
const agBoundaryModule = "example.com/agboundary"

func TestAGBoundaryMechanisms(t *testing.T) {
	lock, err := os.ReadFile(filepath.Join("..", "..", "tools", "toolchain.env"))
	if err != nil {
		t.Fatalf("AG-01 INCOMPLETE: read canonical toolchain lock: %v", err)
	}
	version, err := agBoundaryGoVersion(string(lock))
	if err != nil {
		t.Fatalf("AG-01 INCOMPLETE: %v", err)
	}
	if runtime.Version() != "go"+version {
		t.Fatalf("AG-01 INCOMPLETE: test binary uses %s; lock requires go%s", runtime.Version(), version)
	}
	name := "go"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	goBinary := filepath.Join(runtime.GOROOT(), "bin", name)
	cache := t.TempDir()
	env := agBoundaryEnvironment(os.Environ(), cache)
	out, code, err := agBoundaryRun(t.TempDir(), env, goBinary, "version")
	if err != nil || code != 0 || strings.TrimSpace(out) != "go version go"+version+" "+runtime.GOOS+"/"+runtime.GOARCH {
		t.Fatalf("AG-01 INCOMPLETE: child toolchain does not match lock: code=%d err=%v output=%q", code, err, out)
	}
	t.Logf("AG-01 toolchain=%s target=%s/%s; offline, no inherited workspace or GOFLAGS", runtime.Version(), runtime.GOOS, runtime.GOARCH)

	cases := agBoundaryCases()
	// A legal control must build and execute before any negative is counted.
	if !t.Run("control", func(t *testing.T) {
		agBoundaryExercise(t, cases[0], version, goBinary, env)
	}) {
		t.Fatal("AG-01 INCOMPLETE: legal control failed; no negative case is qualified")
	}
	for _, tc := range cases[1:] {
		t.Run(tc.name, func(t *testing.T) {
			agBoundaryExercise(t, tc, version, goBinary, env)
		})
	}
}

func agBoundaryExercise(t *testing.T, tc agBoundaryCase, version, goBinary string, env []string) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{"go.mod": "module " + agBoundaryModule + "\n\ngo " + version + "\n"}
	for name, source := range tc.files {
		files[name] = source
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		path, err := agBoundaryPath(root, name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(files[name]), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	before, err := agBoundarySnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		after, err := agBoundarySnapshot(root)
		if err != nil || !reflect.DeepEqual(before, after) {
			t.Errorf("AG-01 fixture source changed: err=%v before=%v after=%v", err, before, after)
		}
	}()
	binary := filepath.Join(t.TempDir(), "probe")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	out, code, err := agBoundaryRun(root, env, goBinary, "build", "-o", binary, "./cmd/probe")
	if err != nil {
		t.Fatalf("AG-01 INCOMPLETE: build could not complete: %v\n%s", err, out)
	}
	if tc.diagnostic != "" {
		if err := agBoundaryExpectedFailure(out, code, tc.diagnostic); err != nil {
			t.Fatalf("wrong rejection for %s: %v\n%s", tc.name, err, out)
		}
		t.Logf("expected compiler rejection: %s", tc.diagnostic)
		return
	}
	if code != 0 || strings.TrimSpace(out) != "" {
		t.Fatalf("legal probe build failed: code=%d output=%q", code, out)
	}
	// Execute twice: results, not paths/timings, must be identical. The binaries
	// have no external resources, input, credentials, database or network calls.
	for attempt := 0; attempt < 2; attempt++ {
		out, code, err = agBoundaryRun(root, env, binary)
		if err != nil || code != 0 || strings.TrimSpace(out) != tc.output {
			t.Fatalf("probe attempt %d: code=%d err=%v got=%q want=%q", attempt, code, err, out, tc.output)
		}
	}
	t.Logf("observed twice: %s", tc.output)
}

// Match one actual compiler diagnostic, not an arbitrary non-zero process exit.
// Extra diagnostics, syntax errors, tool failures or unsupported output are not
// accepted even when they happen to contain the expected error's words.
func agBoundaryExpectedFailure(output string, code int, pattern string) error {
	if code != 1 {
		return fmt.Errorf("expected compiler exit 1, got %d", code)
	}
	want, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid diagnostic contract: %w", err)
	}
	position := regexp.MustCompile(`^.+\.go:[0-9]+:[0-9]+: .+$`)
	count := 0
	for _, line := range strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if position.MatchString(line) {
			count++
			if !want.MatchString(line) {
				return fmt.Errorf("unexpected compiler diagnostic: %s", line)
			}
			continue
		}
		if strings.HasPrefix(line, "# "+agBoundaryModule+"/") || strings.HasPrefix(line, "package "+agBoundaryModule+"/") || strings.HasPrefix(line, "imports "+agBoundaryModule+"/") {
			continue
		}
		return fmt.Errorf("unrecognized build output: %s", line)
	}
	if count != 1 {
		return fmt.Errorf("expected exactly one source diagnostic, got %d", count)
	}
	return nil
}

func agBoundaryRun(root string, env []string, binary string, args ...string) (string, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir, cmd.Env = root, env
	cmd.WaitDelay = 2 * time.Second
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return string(out), -1, ctx.Err()
	}
	if err == nil {
		return string(out), 0, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return string(out), exit.ExitCode(), nil
	}
	return string(out), -1, err
}

func agBoundaryGoVersion(lock string) (string, error) {
	var version string
	for _, line := range strings.Split(lock, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok && key == "GO_VERSION" {
			if version != "" || !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(value) {
				return "", fmt.Errorf("invalid or duplicate GO_VERSION in canonical lock")
			}
			version = value
		}
	}
	if version == "" {
		return "", fmt.Errorf("GO_VERSION is missing from canonical lock")
	}
	return version, nil
}

func agBoundaryEnvironment(parent []string, cache string) []string {
	var env []string
	for _, entry := range parent {
		key, _, _ := strings.Cut(entry, "=")
		key = strings.ToUpper(key)
		if strings.HasPrefix(key, "GO") || strings.HasPrefix(key, "CGO_") {
			continue
		}
		env = append(env, entry)
	}
	return append(env,
		"GOENV=off", "GOTOOLCHAIN=local", "GOWORK=off", "GO111MODULE=on",
		"GOFLAGS=-mod=readonly -buildvcs=false", "GOPROXY=off", "GOSUMDB=off", "GOVCS=*:off",
		"CGO_ENABLED=0", "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH,
		"GOROOT="+runtime.GOROOT(), "GOPATH="+filepath.Join(cache, "gopath"), "GOCACHE="+filepath.Join(cache, "build"))
}

func agBoundaryPath(root, name string) (string, error) {
	if name == "" || name == "." || strings.ContainsAny(name, "\\:") || filepath.IsAbs(name) || filepath.ToSlash(filepath.Clean(name)) != name || name == ".." || strings.HasPrefix(name, "../") {
		return "", fmt.Errorf("invalid fixture path %q", name)
	}
	return filepath.Join(root, filepath.FromSlash(name)), nil
}

func agBoundarySnapshot(root string) (map[string]string, error) {
	result := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("unexpected non-regular fixture entry: %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(name)] = fmt.Sprintf("%x", sha256.Sum256(data))
		return nil
	})
	return result, err
}

func TestAGBoundaryHarnessRejectsFalsePass(t *testing.T) {
	pattern := `^internal/b/b\.go:[0-9]+:[0-9]+: use of internal package example\.com/agboundary/internal/a/internal/store not allowed$`
	valid := "internal/b/b.go:2:8: use of internal package example.com/agboundary/internal/a/internal/store not allowed\n"
	if err := agBoundaryExpectedFailure(valid, 1, pattern); err != nil {
		t.Fatal(err)
	}
	for name, output := range map[string]string{
		"empty": "", "marker_only": "use of internal package not allowed", "syntax": "internal/b/b.go:2:8: syntax error\n",
		"mixed": valid + "internal/b/b.go:3:1: syntax error\n", "duplicate": valid + valid,
		"infrastructure": "go: download failed\n" + valid,
		"wrong_target": strings.ReplaceAll(valid, "/a/internal/", "/unrelated/internal/"),
	} {
		t.Run(name, func(t *testing.T) {
			if err := agBoundaryExpectedFailure(output, 1, pattern); err == nil {
				t.Fatal("false pass accepted")
			}
		})
	}
	for _, code := range []int{-1, 0, 2, 124, 137} {
		if err := agBoundaryExpectedFailure(valid, code, pattern); err == nil {
			t.Fatalf("unexpected exit code %d accepted", code)
		}
	}
}

func TestAGBoundaryHarnessInputs(t *testing.T) {
	for _, input := range []string{"", "GO_VERSION=latest", "GO_VERSION=1.25.13\nGO_VERSION=1.25.13", "GO_VERSION=1.25.13; echo bad"} {
		if _, err := agBoundaryGoVersion(input); err == nil {
			t.Fatalf("invalid lock accepted: %q", input)
		}
	}
	if version, err := agBoundaryGoVersion("# lock\nGO_VERSION=1.25.13\n"); err != nil || version != "1.25.13" {
		t.Fatalf("valid lock: %q %v", version, err)
	}
	for _, path := range []string{"", ".", "..", "../escape.go", "/tmp/x", "a/../../x", "a/./x", `a\x`, "C:/x"} {
		if _, err := agBoundaryPath(t.TempDir(), path); err == nil {
			t.Fatalf("invalid fixture path accepted: %q", path)
		}
	}
	if _, err := agBoundaryPath(t.TempDir(), "internal/a/build.go"); err != nil {
		t.Fatal(err)
	}
	env := agBoundaryEnvironment([]string{"PATH=/bin", "GOWORK=/bad", "GOFLAGS=-tags=bad", "GOENV=/bad", "GOTOOLCHAIN=auto", "CGO_ENABLED=1", "GOOS=bad", "GOEXPERIMENT=bad"}, "/cache")
	joined := strings.Join(env, "\n")
	for _, unwanted := range []string{"/bad", "-tags=bad", "=auto", "CGO_ENABLED=1", "GOOS=bad", "GOEXPERIMENT=bad"} {
		if strings.Contains(joined, unwanted) {
			t.Errorf("inherited unsafe environment: %s", unwanted)
		}
	}
	for _, required := range []string{"GOWORK=off", "GOTOOLCHAIN=local", "GOPROXY=off", "GOENV=off", "CGO_ENABLED=0"} {
		if !strings.Contains(joined, required) {
			t.Errorf("missing environment: %s", required)
		}
	}
}
