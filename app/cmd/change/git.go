package change

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

type gitCommandError struct {
	Args   []string
	Output string
	Err    error
}

func (failure *gitCommandError) Error() string {
	if failure == nil {
		return "git command failed"
	}
	detail := strings.TrimSpace(failure.Output)
	if detail == "" && failure.Err != nil {
		detail = failure.Err.Error()
	}
	if detail == "" {
		detail = "unknown git failure"
	}
	return fmt.Sprintf("git %s: %s", strings.Join(failure.Args, " "), detail)
}

func (failure *gitCommandError) Unwrap() error {
	if failure == nil {
		return nil
	}
	return failure.Err
}

func runGit(root string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		output := strings.TrimSpace(stderr.String())
		if output == "" {
			output = strings.TrimSpace(stdout.String())
		}
		return "", &gitCommandError{Args: append([]string(nil), args...), Output: output, Err: err}
	}
	return strings.TrimSpace(stdout.String()), nil
}

func resolveGitBase(root, base string) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "HEAD"
	}
	value, err := runGit(root, "rev-parse", "--verify", base+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("change contract: resolve base %q: %w", base, err)
	}
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("change contract: resolve base %q returned an empty commit identity", base)
	}
	return strings.TrimSpace(value), nil
}

func ensureCleanWorktree(root string) error {
	value, err := runGit(root, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return fmt.Errorf("change contract: inspect worktree: %w", err)
	}
	if strings.TrimSpace(value) != "" {
		return fmt.Errorf("change contract: worktree must be clean before `yunka change begin`; commit/stash unrelated changes first")
	}
	return nil
}
