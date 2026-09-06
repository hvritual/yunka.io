package change

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestIssue160SourceSnapshotUsesExactGitBlobs(t *testing.T) {
	repository := t.TempDir()
	root := filepath.Join(repository, "backend")
	writePressureFile(t, filepath.Join(root, "contracts/api.proto"), "literal $Format:%H$\n")
	writePressureFile(t, filepath.Join(root, ".gitattributes"), "contracts/api.proto export-ignore export-subst\n")
	writePressureFile(t, filepath.Join(repository, "outside.txt"), "must not be copied\n")
	base := commitNestedGitTest(t, repository)
	writePressureFile(t, filepath.Join(root, "contracts/api.proto"), "uncommitted different source\n")
	originalStatus := gitPressure(t, repository, "status", "--porcelain")
	snapshot, cleanup, err := materializeSourceBase(context.Background(), root, base)
	if err != nil {
		t.Fatal(err)
	}
	if got := readPressureFile(t, filepath.Join(snapshot, "contracts/api.proto")); got != "literal $Format:%H$\n" {
		t.Fatalf("base blob transformed: %q", got)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(snapshot), "outside.txt")); !os.IsNotExist(err) {
		t.Fatal("unrelated repository subtree copied")
	}
	if _, err := os.Stat(filepath.Join(snapshot, ".git")); !os.IsNotExist(err) {
		t.Fatal("snapshot contains Git state")
	}
	if got := gitPressure(t, repository, "status", "--porcelain"); got != originalStatus {
		t.Fatal("materialization changed caller Git state")
	}
	cleanup()
	if _, err := os.Stat(snapshot); !os.IsNotExist(err) {
		t.Fatal("snapshot not removed")
	}
}

func TestIssue160SourceSnapshotRejectsEscapeAndHonorsCancellation(t *testing.T) {
	root := t.TempDir()
	writePressureFile(t, filepath.Join(root, "api.proto"), "baseline")
	base := commitNestedGitTest(t, root)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, cleanup, err := materializeSourceBase(ctx, root, base); err == nil {
		cleanup()
		t.Fatal("cancelled snapshot accepted")
	}
	for _, path := range []string{"../outside", "/absolute", "a/../b", ".git/config", "a/.GIT/config", "C:/windows", "a\\b"} {
		if safeSnapshotPath(path) {
			t.Fatalf("accepted unsafe path %q", path)
		}
	}
	if runtime.GOOS == "windows" {
		return
	}
	outside := t.TempDir()
	sentinel := filepath.Join(outside, "sentinel")
	writePressureFile(t, sentinel, "unchanged")
	if err := os.Symlink(sentinel, filepath.Join(root, "link.proto")); err != nil {
		t.Fatal(err)
	}
	gitPressure(t, root, "add", "link.proto")
	gitPressure(t, root, "commit", "-m", "escape fixture")
	base = gitPressure(t, root, "rev-parse", "HEAD")
	if _, cleanup, err := materializeSourceBase(context.Background(), root, base); err == nil {
		cleanup()
		t.Fatal("escaping symlink accepted")
	} else if !strings.Contains(err.Error(), "escapes snapshot") {
		t.Fatal(err)
	}
	if readPressureFile(t, sentinel) != "unchanged" {
		t.Fatal("outside file changed")
	}
}
