package fastfeedback

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFingerprintRootsIsDeterministicAndHostPathFree(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "a", "one.txt"), "one")
	writeTestFile(t, filepath.Join(root, "a", "two.txt"), "two")
	writeTestFile(t, filepath.Join(root, "b.txt"), "three")

	first, err := FingerprintRoots([]Root{
		{Label: "second", Path: filepath.Join(root, "b.txt")},
		{Label: "first", Path: filepath.Join(root, "a")},
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := FingerprintRoots([]Root{
		{Label: "first", Path: filepath.Join(root, "a")},
		{Label: "second", Path: filepath.Join(root, "b.txt")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("fingerprint depends on root order: first=%#v second=%#v", first, second)
	}
	if first.Files != 3 || first.Bytes != int64(len("one")+len("two")+len("three")) {
		t.Fatalf("unexpected counts: %#v", first)
	}
	if strings.Contains(first.Digest, root) {
		t.Fatalf("digest leaked host path %q", root)
	}
}

func TestFingerprintChangesOnContentAndRename(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "input", "item.txt")
	writeTestFile(t, path, "alpha")
	baseline, err := FingerprintRoots([]Root{{Label: "input", Path: filepath.Dir(path)}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("beta"), 0o640); err != nil {
		t.Fatal(err)
	}
	contentChanged, err := FingerprintRoots([]Root{{Label: "input", Path: filepath.Dir(path)}})
	if err != nil {
		t.Fatal(err)
	}
	if baseline.Digest == contentChanged.Digest {
		t.Fatal("content change did not invalidate digest")
	}
	if err := os.Rename(path, filepath.Join(filepath.Dir(path), "renamed.txt")); err != nil {
		t.Fatal(err)
	}
	renamed, err := FingerprintRoots([]Root{{Label: "input", Path: filepath.Dir(path)}})
	if err != nil {
		t.Fatal(err)
	}
	if contentChanged.Digest == renamed.Digest {
		t.Fatal("file rename did not invalidate digest")
	}
}

func TestOptionalMissingRootIsPartOfFingerprint(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "optional")
	before, err := FingerprintRoots([]Root{{Label: "optional", Path: missing, Optional: true}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(missing, 0o750); err != nil {
		t.Fatal(err)
	}
	after, err := FingerprintRoots([]Root{{Label: "optional", Path: missing, Optional: true}})
	if err != nil {
		t.Fatal(err)
	}
	if before.Digest == after.Digest {
		t.Fatal("missing->present transition did not invalidate digest")
	}
}

func TestDirectorySymlinkCannotProvideReusableEvidence(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.MkdirAll(target, 0o750); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(target, "item.txt"), "value")
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	_, err := FingerprintRoots([]Root{{Label: "linked", Path: link}})
	if err == nil || !strings.Contains(err.Error(), "directory symlink") {
		t.Fatalf("expected directory symlink rejection, got %v", err)
	}
}

func TestMetadataRoundTripIsDeterministic(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "input.txt"), "input")
	writeTestFile(t, filepath.Join(root, "output.txt"), "output")
	inputs, err := FingerprintRoots([]Root{{Label: "inputs", Path: filepath.Join(root, "input.txt")}})
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := FingerprintRoots([]Root{{Label: "outputs", Path: filepath.Join(root, "output.txt")}})
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := NewMetadata(EngineIdentity{ID: "vcs:abc123", Verified: true}, "protoc:sha256:tool", inputs, outputs)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "cache.json")
	if err := Write(path, metadata); err != nil {
		t.Fatal(err)
	}
	firstBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(metadata, loaded) {
		t.Fatalf("round trip drift: want=%#v got=%#v", metadata, loaded)
	}
	if err := Write(path, metadata); err != nil {
		t.Fatal(err)
	}
	secondBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(firstBytes, secondBytes) {
		t.Fatal("metadata serialization is not deterministic")
	}
	if strings.Contains(string(firstBytes), root) || strings.Contains(string(firstBytes), "timestamp") {
		t.Fatalf("cache leaked host-specific/volatile data: %s", firstBytes)
	}
}

func TestReusableRequiresVerifiedExactEvidence(t *testing.T) {
	fingerprint := Fingerprint{Digest: strings.Repeat("a", 64), Files: 1, Bytes: 4}
	base, err := NewMetadata(EngineIdentity{ID: "vcs:one", Verified: true}, "tool:one", fingerprint, fingerprint)
	if err != nil {
		t.Fatal(err)
	}
	if !Reusable(base, base) {
		t.Fatal("exact verified metadata should be reusable")
	}
	unverified := base
	unverified.Engine.Verified = false
	if Reusable(base, unverified) {
		t.Fatal("unverified engine must not be reusable")
	}
	changedEngine := base
	changedEngine.Engine.ID = "vcs:two"
	if Reusable(base, changedEngine) {
		t.Fatal("engine mismatch must not be reusable")
	}
	changedTool := base
	changedTool.Toolchain = "tool:two"
	if Reusable(base, changedTool) {
		t.Fatal("toolchain mismatch must not be reusable")
	}
	changedInput := base
	changedInput.Inputs.Digest = strings.Repeat("b", 64)
	if Reusable(base, changedInput) {
		t.Fatal("input mismatch must not be reusable")
	}
	changedOutput := base
	changedOutput.Outputs.Digest = strings.Repeat("c", 64)
	if Reusable(base, changedOutput) {
		t.Fatal("output mismatch must not be reusable")
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "cache.json")
	contents := `{"schemaVersion":1,"algorithm":"sha256-tree-v1","engine":{"id":"vcs:x","verified":true},"toolchain":"tool","inputs":{"digest":"` + strings.Repeat("a", 64) + `","files":0,"bytes":0},"outputs":{"digest":"` + strings.Repeat("b", 64) + `","files":0,"bytes":0},"unexpected":true}`
	if err := os.WriteFile(path, []byte(contents), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected unknown cache field to be rejected")
	}
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o640); err != nil {
		t.Fatal(err)
	}
}
