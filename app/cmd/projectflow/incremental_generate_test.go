package projectflow

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"yunka.io/pkg/fastfeedback"
)

func TestIncrementalGenerateExactVerifiedHitBypassesCanonicalGenerateAndWritesNothing(t *testing.T) {
	root, options, engine, cachePath := prepareFastCheckEvidenceFixture(t)
	manifestPath := filepath.Join(root, "contracts", "generated", "manifest.json")
	generatedGoPath := filepath.Join(root, "internal", "generated.txt")

	beforeCache := fileSnapshotForGenerate(t, cachePath)
	beforeManifest := fileSnapshotForGenerate(t, manifestPath)
	beforeGeneratedGo := fileSnapshotForGenerate(t, generatedGoPath)
	called := false

	report, err := generateIncremental(
		context.Background(),
		options,
		false,
		engine,
		func(context.Context, string) (string, error) { return fastCheckToolchain, nil },
		func(context.Context, Options) (Report, error) {
			called = true
			return Report{}, errors.New("canonical generate must not run on exact hit")
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("canonical generate ran on exact verified evidence hit")
	}
	if report.Root != root || len(report.Stages) != 1 || report.Stages[0].Name != "fast-generate" || report.Stages[0].Status != "unchanged" {
		t.Fatalf("unexpected fast generate report: %#v", report)
	}

	assertGenerateSnapshotEqual(t, beforeCache, fileSnapshotForGenerate(t, cachePath), "cache")
	assertGenerateSnapshotEqual(t, beforeManifest, fileSnapshotForGenerate(t, manifestPath), "manifest")
	assertGenerateSnapshotEqual(t, beforeGeneratedGo, fileSnapshotForGenerate(t, generatedGoPath), "generated Go")
}

func TestIncrementalGenerateUnsafeEvidenceAlwaysFallsBack(t *testing.T) {
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
				if err := os.WriteFile(cachePath, []byte("{broken"), 0o640); err != nil {
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
			toolchain: func(context.Context, string) (string, error) { return "protoc:different", nil },
		},
		{
			name: "toolchain capture failure",
			toolchain: func(context.Context, string) (string, error) { return "", errors.New("tool unavailable") },
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
			if test.engine != nil {
				engine = test.engine(engine)
			}
			toolchain := test.toolchain
			if toolchain == nil {
				toolchain = func(context.Context, string) (string, error) { return fastCheckToolchain, nil }
			}
			called := 0
			want := Report{Root: root, Stages: []Stage{{Name: "canonical-generate", Status: "generated"}}}
			report, err := generateIncremental(
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
				t.Fatalf("canonical generate calls=%d want 1", called)
			}
			if !reflect.DeepEqual(report, want) {
				t.Fatalf("fallback report=%#v want %#v", report, want)
			}
		})
	}
}

func TestIncrementalGenerateOutputDriftRunsCanonicalGenerationAndRecovers(t *testing.T) {
	protoc, err := exec.LookPath("protoc")
	if err != nil {
		t.Skip("protoc is required for C11.4-C canonical fallback integration")
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

	manifestPath := filepath.Join(root, "contracts", "generated", "manifest.json")
	contents, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, append(contents, '\n'), 0o640); err != nil {
		t.Fatal(err)
	}

	called := 0
	report, err := generateIncremental(
		context.Background(),
		options,
		false,
		engine,
		func(context.Context, string) (string, error) { return fastCheckToolchain, nil },
		func(ctx context.Context, opts Options) (Report, error) {
			called++
			return GenerateWithFastFeedback(ctx, opts)
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatalf("canonical generation calls=%d want 1", called)
	}
	if len(report.Stages) == 0 || report.Stages[0].Name == "fast-generate" {
		t.Fatalf("expected canonical generation report, got %#v", report)
	}
	if _, err := Check(context.Background(), options); err != nil {
		t.Fatalf("canonical fallback did not recover generated output: %v", err)
	}
	if _, err := fastfeedback.Load(cachePath); err != nil {
		t.Fatalf("canonical fallback did not leave valid disposable evidence: %v", err)
	}
}

type generateFileSnapshot struct {
	contents []byte
	mode     os.FileMode
	modUnix  int64
	size     int64
}

func fileSnapshotForGenerate(t *testing.T, path string) generateFileSnapshot {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return generateFileSnapshot{
		contents: contents,
		mode:     info.Mode(),
		modUnix:  info.ModTime().UnixNano(),
		size:     info.Size(),
	}
}

func assertGenerateSnapshotEqual(t *testing.T, before, after generateFileSnapshot, name string) {
	t.Helper()
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("%s changed on exact no-op hit:\nbefore=%#v\nafter=%#v", name, before, after)
	}
}
