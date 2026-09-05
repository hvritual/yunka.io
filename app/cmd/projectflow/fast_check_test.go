package projectflow

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/hvritual/yunka.io/pkg/fastfeedback"
)

const fastCheckToolchain = "protoc:test:sha256:verified"

func TestFastCheckExactVerifiedHitBypassesCanonicalCheck(t *testing.T) {
	root, options, engine, cachePath := prepareFastCheckEvidenceFixture(t)
	before := readTestFile(t, cachePath)
	called := false
	report, err := checkWithFastFeedback(
		context.Background(),
		options,
		false,
		engine,
		func(context.Context, string) (string, error) { return fastCheckToolchain, nil },
		func(context.Context, Options) (Report, error) {
			called = true
			return Report{}, errors.New("canonical check must not run on exact hit")
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("canonical check ran on exact verified evidence hit")
	}
	if report.Root != root || len(report.Stages) != 1 || report.Stages[0].Name != "fast-check" || report.Stages[0].Status != "ok" {
		t.Fatalf("unexpected fast report: %#v", report)
	}
	after := readTestFile(t, cachePath)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("fast check mutated cache on hit")
	}
}

func TestFastCheckExactVerifiedHitCannotBypassLegacyModuleIdentity(t *testing.T) {
	root, options, engine, cachePath := prepareFastCheckEvidenceFixture(t)
	writeTestFile(t, filepath.Join(root, "internal", "legacy.go"), "package internal\nimport _ \"yunka.io/framework/core/modulecatalog\"\n")
	project, err := resolveProject(options)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := buildFastFeedbackMetadataWithIdentity(project, engine, fastCheckToolchain)
	if err != nil {
		t.Fatal(err)
	}
	if err := fastfeedback.Write(cachePath, metadata); err != nil {
		t.Fatal(err)
	}
	called := false
	_, err = checkWithFastFeedback(
		context.Background(),
		options,
		false,
		engine,
		func(context.Context, string) (string, error) { return fastCheckToolchain, nil },
		func(context.Context, Options) (Report, error) {
			called = true
			return Report{}, errors.New("canonical check should not be needed to reject module identity drift")
		},
	)
	if err == nil {
		t.Fatal("verified fast-feedback cache bypassed legacy module identity drift")
	}
	var failure *Failure
	if !errors.As(err, &failure) || failure.Kind != FailureModuleIdentity {
		t.Fatalf("expected module identity failure, got %T %v", err, err)
	}
	if called {
		t.Fatal("module identity drift should be rejected before cache reuse or canonical fallback")
	}
}

func TestFastCheckUnsafeEvidenceAlwaysFallsBackReadOnly(t *testing.T) {
	tests := []struct {
		name      string
		forceFull bool
		mutate    func(t *testing.T, root, cachePath string)
		engine    func(fastfeedback.EngineIdentity) fastfeedback.EngineIdentity
		toolchain func(context.Context, string) (string, error)
	}{
		{
			name: "missing cache",
			mutate: func(t *testing.T, _, cachePath string) {
				if err := os.Remove(cachePath); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "corrupt cache",
			mutate: func(t *testing.T, _, cachePath string) {
				if err := os.WriteFile(cachePath, []byte("{not-json"), 0o640); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "unverified engine",
			engine: func(value fastfeedback.EngineIdentity) fastfeedback.EngineIdentity {
				value.Verified = false
				return value
			},
		},
		{
			name: "engine mismatch",
			engine: func(value fastfeedback.EngineIdentity) fastfeedback.EngineIdentity {
				value.ID = "vcs:different"
				return value
			},
		},
		{
			name: "toolchain mismatch",
			toolchain: func(context.Context, string) (string, error) {
				return "protoc:different", nil
			},
		},
		{
			name: "toolchain capture failure",
			toolchain: func(context.Context, string) (string, error) {
				return "", errors.New("tool unavailable")
			},
		},
		{
			name: "input mismatch",
			mutate: func(t *testing.T, root, _ string) {
				writeTestFile(t, filepath.Join(root, "contracts", "proto", "input.proto"), "changed")
			},
		},
		{
			name: "output mismatch",
			mutate: func(t *testing.T, root, _ string) {
				writeTestFile(t, filepath.Join(root, "contracts", "generated", "manifest.json"), "changed")
			},
		},
		{
			name:      "force full",
			forceFull: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root, options, engine, cachePath := prepareFastCheckEvidenceFixture(t)
			if test.mutate != nil {
				test.mutate(t, root, cachePath)
			}
			before, beforeErr := os.ReadFile(cachePath)
			if test.engine != nil {
				engine = test.engine(engine)
			}
			toolchain := test.toolchain
			if toolchain == nil {
				toolchain = func(context.Context, string) (string, error) { return fastCheckToolchain, nil }
			}
			called := 0
			want := Report{Root: root, Stages: []Stage{{Name: "canonical", Status: "ok"}}}
			report, err := checkWithFastFeedback(
				context.Background(),
				options,
				test.forceFull,
				engine,
				toolchain,
				func(context.Context, Options) (Report, error) {
					called++
					return want, nil
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			if called != 1 {
				t.Fatalf("canonical check calls=%d want 1", called)
			}
			if !reflect.DeepEqual(report, want) {
				t.Fatalf("fallback report=%#v want %#v", report, want)
			}
			after, afterErr := os.ReadFile(cachePath)
			if errors.Is(beforeErr, os.ErrNotExist) {
				if !errors.Is(afterErr, os.ErrNotExist) {
					t.Fatalf("missing cache was created by check: %v", afterErr)
				}
			} else {
				if beforeErr != nil || afterErr != nil {
					t.Fatalf("cache read before=%v after=%v", beforeErr, afterErr)
				}
				if !reflect.DeepEqual(before, after) {
					t.Fatal("fallback check mutated cache")
				}
			}
		})
	}
}

func TestFastCheckOutputDriftFallsBackToCanonicalDriftDetection(t *testing.T) {
	protoc, err := exec.LookPath("protoc")
	if err != nil {
		t.Skip("protoc is required for C11.4-B canonical fallback integration")
	}
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "contracts", "proto", "simple.proto"), `syntax = "proto3";
package simple.v1;
message Item { string id = 1; }
`)
	options := Options{Root: root, Protoc: protoc}
	if _, err := Generate(context.Background(), options); err != nil {
		t.Fatal(err)
	}
	project, err := resolveProject(options)
	if err != nil {
		t.Fatal(err)
	}
	engine := fastfeedback.EngineIdentity{ID: "vcs:test", Verified: true}
	metadata, err := buildFastFeedbackMetadataWithIdentity(project, engine, fastCheckToolchain)
	if err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(root, filepath.FromSlash(fastfeedback.CacheRelativePath))
	if err := fastfeedback.Write(cachePath, metadata); err != nil {
		t.Fatal(err)
	}
	beforeCache := readTestFile(t, cachePath)
	manifestPath := filepath.Join(root, "contracts", "generated", "manifest.json")
	contents := readTestFile(t, manifestPath)
	if err := os.WriteFile(manifestPath, append(contents, '\n'), 0o640); err != nil {
		t.Fatal(err)
	}
	_, err = checkWithFastFeedback(
		context.Background(),
		options,
		false,
		engine,
		func(context.Context, string) (string, error) { return fastCheckToolchain, nil },
		Check,
	)
	if err == nil {
		t.Fatal("expected canonical drift failure after output fingerprint mismatch")
	}
	var failure *Failure
	if !errors.As(err, &failure) || failure.Kind != FailureContractDrift {
		t.Fatalf("expected contract drift failure, got %T %v", err, err)
	}
	afterCache := readTestFile(t, cachePath)
	if !reflect.DeepEqual(beforeCache, afterCache) {
		t.Fatal("drift fallback mutated cache")
	}
}

func prepareFastCheckEvidenceFixture(t *testing.T) (string, Options, fastfeedback.EngineIdentity, string) {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/fastcheck\n\ngo 1.25.0\n")
	writeTestFile(t, filepath.Join(root, "contracts", "proto", "input.proto"), "input")
	writeTestFile(t, filepath.Join(root, "contracts", "generated", "manifest.json"), "generated")
	writeTestFile(t, filepath.Join(root, "internal", "generated.txt"), "generated-go")
	options := Options{Root: root}
	project, err := resolveProject(options)
	if err != nil {
		t.Fatal(err)
	}
	engine := fastfeedback.EngineIdentity{ID: "vcs:test", Verified: true}
	metadata, err := buildFastFeedbackMetadataWithIdentity(project, engine, fastCheckToolchain)
	if err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(root, filepath.FromSlash(fastfeedback.CacheRelativePath))
	if err := fastfeedback.Write(cachePath, metadata); err != nil {
		t.Fatal(err)
	}
	return root, options, engine, cachePath
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return contents
}
