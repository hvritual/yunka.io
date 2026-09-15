package auditcore

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"yunka.io/app/cmd/gitproject"
)

// MaterializeGitCommit copies the exact tracked project tree at commitSHA into
// an isolated temporary directory. The repository working tree is never used as
// a mutation target, so current and baseline audits can execute identical
// read-only rule paths.
func MaterializeGitCommit(projectRoot, commitSHA string) (string, func(), error) {
	commitSHA = strings.TrimSpace(commitSHA)
	if commitSHA == "" {
		return "", nil, fmt.Errorf("audit debt: base commit SHA is required")
	}
	paths, err := gitproject.Resolve(projectRoot)
	if err != nil {
		return "", nil, fmt.Errorf("audit debt: resolve Git project paths: %w", err)
	}
	args := []string{"archive", "--format=tar", commitSHA}
	if paths.ProjectPrefix != "." {
		args = append(args, "--", paths.ProjectPrefix)
	}
	archive, err := runGitBytes(paths.RepositoryRoot, args...)
	if err != nil {
		return "", nil, fmt.Errorf("audit debt: archive baseline %s: %w", commitSHA, err)
	}
	temporary, err := os.MkdirTemp("", "yunka-audit-baseline-")
	if err != nil {
		return "", nil, fmt.Errorf("audit debt: create baseline workspace: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(temporary) }
	reader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			cleanup()
			return "", nil, fmt.Errorf("audit debt: read baseline archive: %w", err)
		}
		if header.Typeflag == tar.TypeXGlobalHeader || header.Typeflag == tar.TypeXHeader {
			continue
		}
		name := filepath.Clean(filepath.FromSlash(header.Name))
		if name == "." || filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
			cleanup()
			return "", nil, fmt.Errorf("audit debt: unsafe baseline archive path %q", header.Name)
		}
		target := filepath.Join(temporary, name)
		relative, err := filepath.Rel(temporary, target)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			cleanup()
			return "", nil, fmt.Errorf("audit debt: baseline archive path escaped workspace: %q", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				cleanup()
				return "", nil, err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				cleanup()
				return "", nil, err
			}
			contents, err := io.ReadAll(reader)
			if err != nil {
				cleanup()
				return "", nil, err
			}
			mode := os.FileMode(header.Mode) & 0o777
			if mode == 0 {
				mode = 0o644
			}
			if err := os.WriteFile(target, contents, mode); err != nil {
				cleanup()
				return "", nil, err
			}
		default:
			cleanup()
			return "", nil, fmt.Errorf("audit debt: unsupported baseline archive entry %q type=%d", header.Name, header.Typeflag)
		}
	}
	baselineRoot := temporary
	if paths.ProjectPrefix != "." {
		baselineRoot = filepath.Join(temporary, filepath.FromSlash(paths.ProjectPrefix))
	}
	info, err := os.Stat(baselineRoot)
	if err != nil || !info.IsDir() {
		cleanup()
		if err == nil {
			err = fmt.Errorf("materialized project root is not a directory")
		}
		return "", nil, fmt.Errorf("audit debt: materialized baseline project: %w", err)
	}
	return baselineRoot, cleanup, nil
}
