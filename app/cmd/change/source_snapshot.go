package change

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"yunka.io/app/cmd/gitproject"
)

// materializeSourceBase reads Git objects, not checkout files or git archive
// projections (which may apply export-ignore/export-subst). It never creates a
// worktree, checks out a branch, initializes submodules or executes base code.
// The private snapshot is disposable compiler input and contains no .git state.
func materializeSourceBase(ctx context.Context, root, base string) (string, func(), error) {
	paths, err := gitproject.Resolve(root)
	if err != nil {
		return "", nil, err
	}
	sha, err := resolveGitBase(root, base)
	if err != nil {
		return "", nil, err
	}
	command := exec.CommandContext(ctx, "git", "-C", paths.RepositoryRoot, "ls-tree", "-rz", "--full-tree", sha)
	command.Env = append(os.Environ(), "GIT_NO_REPLACE_OBJECTS=1")
	tree, err := command.Output()
	if err != nil {
		return "", nil, fmt.Errorf("contract sources: read base tree: %w", err)
	}
	directory, err := os.MkdirTemp("", "yunka-source-base-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(directory) }
	success := false
	defer func() {
		if !success {
			cleanup()
		}
	}()
	type entry struct{ mode, sha, name string }
	entries := []entry{}
	var input strings.Builder
	for _, record := range bytes.Split(tree, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		metadata, name, ok := strings.Cut(string(record), "\t")
		fields := strings.Fields(metadata)
		if !ok || len(fields) != 3 {
			return "", nil, fmt.Errorf("contract sources: malformed Git tree record")
		}
		if fields[1] == "commit" {
			continue
		} // gitlink: never fetch or invent its contents
		if fields[1] != "blob" || (fields[0] != "100644" && fields[0] != "100755" && fields[0] != "120000") {
			return "", nil, fmt.Errorf("contract sources: unsupported Git mode %s", fields[0])
		}
		if !safeSnapshotPath(name) {
			return "", nil, fmt.Errorf("contract sources: unsafe Git path %q", name)
		}
		// Only this project's subtree is needed. Inventory roots are required
		// to be project-relative. Explicit external includes remain external.
		if _, inside, err := paths.ToProject(name); err != nil || !inside {
			continue
		}
		entries = append(entries, entry{fields[0], fields[2], name})
		input.WriteString(fields[2] + "\n")
	}
	cmd := exec.CommandContext(ctx, "git", "-C", paths.RepositoryRoot, "cat-file", "--batch")
	cmd.Env = append(os.Environ(), "GIT_NO_REPLACE_OBJECTS=1")
	cmd.Stdin = strings.NewReader(input.String())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", nil, err
	}
	if err := cmd.Start(); err != nil {
		return "", nil, err
	}
	waited := false
	defer func() {
		if !waited {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	reader := bufio.NewReader(stdout)
	links := []struct{ path, target string }{}
	for _, item := range entries {
		header, err := reader.ReadString('\n')
		if err != nil {
			return "", nil, err
		}
		fields := strings.Fields(header)
		if len(fields) != 3 || fields[0] != item.sha || fields[1] != "blob" {
			return "", nil, fmt.Errorf("contract sources: unexpected Git object header")
		}
		size, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil || size < 0 {
			return "", nil, fmt.Errorf("contract sources: invalid Git object size")
		}
		destination := filepath.Join(directory, filepath.FromSlash(item.name))
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			return "", nil, err
		}
		if item.mode == "120000" {
			contents, err := io.ReadAll(io.LimitReader(reader, size))
			if err != nil || int64(len(contents)) != size {
				return "", nil, fmt.Errorf("contract sources: incomplete Git symlink")
			}
			target := string(contents)
			resolved := filepath.Clean(filepath.Join(filepath.Dir(item.name), filepath.FromSlash(target)))
			if target == "" || filepath.IsAbs(target) || strings.ContainsAny(target, "\\:\x00") || !safeSnapshotPath(filepath.ToSlash(resolved)) {
				return "", nil, fmt.Errorf("contract sources: base symlink %s escapes snapshot", item.name)
			}
			links = append(links, struct{ path, target string }{destination, target})
		} else {
			file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
			if err != nil {
				return "", nil, err
			}
			_, copyErr := io.CopyN(file, reader, size)
			closeErr := file.Close()
			if copyErr != nil {
				return "", nil, copyErr
			}
			if closeErr != nil {
				return "", nil, closeErr
			}
		}
		terminator, err := reader.ReadByte()
		if err != nil || terminator != '\n' {
			return "", nil, fmt.Errorf("contract sources: invalid Git object terminator")
		}
	}
	if _, err := reader.ReadByte(); err != io.EOF {
		return "", nil, fmt.Errorf("contract sources: trailing Git object data")
	}
	err = cmd.Wait()
	waited = true
	if err != nil {
		return "", nil, fmt.Errorf("contract sources: read base blobs: %w: %s", err, stderr.String())
	}
	// No symlink exists while regular files are written. This rules out writes
	// through a crafted parent symlink during snapshot materialization.
	for _, link := range links {
		if err := os.Symlink(link.target, link.path); err != nil {
			return "", nil, err
		}
	}
	project := filepath.Join(directory, filepath.FromSlash(paths.ProjectPrefix))
	if err := os.MkdirAll(project, 0o700); err != nil {
		return "", nil, err
	}
	success = true
	return project, cleanup, nil
}

func safeSnapshotPath(path string) bool {
	if path == "" || path == "." || strings.ContainsAny(path, "\\:\x00") || filepath.IsAbs(path) || filepath.ToSlash(filepath.Clean(path)) != path || path == ".." || strings.HasPrefix(path, "../") {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if strings.EqualFold(part, ".git") {
			return false
		}
	}
	return true
}
