package applicationboundary

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func writeModule(t *testing.T, source string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range map[string]string{"go.mod": "module " + testPackage + "\n\ngo 1.23.0\n", "source.go": definitions + source} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
func fileHashes(t *testing.T, root string) map[string][32]byte {
	t.Helper()
	result := map[string][32]byte{}
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			data, e := os.ReadFile(p)
			if e != nil {
				return e
			}
			rel, e := filepath.Rel(root, p)
			if e != nil {
				return e
			}
			result[rel] = sha256.Sum256(data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func TestModuleCheckReadOnlyAndDeterministic(t *testing.T) {
	root := writeModule(t, `func Build()Reader{return &narrow{}}`)
	before := fileHashes(t, root)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	r := Check(ctx, root, policyFor("Build"), Options{})
	if r.Status != Pass || r.CheckedFactories != 1 || len(r.Sources) != 2 {
		t.Fatalf("legal module: %+v", r)
	}
	a, _ := r.Marshal()
	b, _ := Check(ctx, root, policyFor("Build"), Options{}).Marshal()
	if string(a) != string(b) {
		t.Fatal("same source differs")
	}
	if !reflect.DeepEqual(before, fileHashes(t, root)) {
		t.Fatal("checker mutated source")
	}
	if strings.Contains(string(a), root) {
		t.Fatal("absolute checkout path leaked into report")
	}
}
func TestModuleErrorsCannotMasqueradeAsClean(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	bad := writeModule(t, `func Broken( {`)
	if r := Check(ctx, bad, policyFor("Build"), Options{}); r.Status != Incomplete {
		t.Fatalf("broken source passed: %+v", r)
	}
	if r := Check(ctx, t.TempDir(), policyFor("Build"), Options{}); r.Status != Incomplete {
		t.Fatal("missing module passed")
	}
	cancelled, cancelNow := context.WithCancel(ctx)
	cancelNow()
	if r := Check(cancelled, writeModule(t, `func Build()Reader{return &narrow{}}`), policyFor("Build"), Options{}); r.Status != Incomplete {
		t.Fatal("cancelled analysis passed")
	}
}
func TestLoaderDoesNotInheritCommandInjection(t *testing.T) {
	for k, v := range map[string]string{"GOFLAGS": "-overlay=/missing/secret", "GOWORK": "/missing/work", "GOTOOLCHAIN": "auto", "GOPACKAGESDRIVER": "/missing/driver"} {
		t.Setenv(k, v)
	}
	r := Check(context.Background(), writeModule(t, `func Build()Reader{return &narrow{}}`), policyFor("Build"), Options{})
	if r.Status != Pass {
		t.Fatalf("inherited unapproved driver/workspace: %+v", r)
	}
}

func TestModuleTestOnlyPackagesAreExplicitlyExcluded(t *testing.T) {
	root := writeModule(t, `func Build()Reader{return &narrow{}}`)
	testdir := filepath.Join(root, "testonly")
	if err := os.Mkdir(testdir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(testdir, "only_test.go"), []byte("package testonly\nimport \"testing\"\nfunc TestNothing(t *testing.T){}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r := Check(context.Background(), root, policyFor("Build"), Options{})
	if r.Status != Pass {
		t.Fatalf("test-only package invalidated production analysis: %+v", r)
	}
	// Go package loaders may omit test-only directories entirely. When returned,
	// these entries must be explicit exclusions, never silently claimed checked.
	for _, e := range r.ExcludedPackages {
		if e.Package != testPackage+"/testonly" || e.Reason != "no-active-production-go-files" {
			t.Fatalf("unexpected exclusion: %+v", e)
		}
	}
	policy := policyFor("Build")
	policy.Factories[0].Symbol.Package = testPackage + "/testonly"
	if r := Check(context.Background(), root, policy, Options{}); r.Status != Incomplete {
		t.Fatal("selected excluded subject passed")
	}
}
