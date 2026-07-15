package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Run(repoPath string, args ...string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "git", args...) //nolint:gosec // args are constructed internally, not from user input
	cmd.Dir = repoPath

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}

	return strings.TrimSpace(string(out)), nil
}

// EnsurePathIsRepo ensures the given path contains a clean git repository.
// If the directory doesn't exist or is empty, it clones from cloneURL.
// It verifies a .git directory exists and aborts if there are uncommitted changes.
func EnsurePathIsRepo(repoPath, cloneURL string) error {
	// ensure repo path exists, cloning if the directory is empty or doesn't exist
	needsClone := false
	if info, err := os.Stat(repoPath); os.IsNotExist(err) {
		if err := os.MkdirAll(repoPath, 0o755); err != nil { //nolint:gosec // directory for user-provided --local-repo-path
			return fmt.Errorf("creating repo path %s: %w", repoPath, err)
		}
		needsClone = true
	} else if err != nil {
		return fmt.Errorf("checking repo path %s: %w", repoPath, err)
	} else if info.IsDir() {
		entries, err := os.ReadDir(repoPath)
		if err != nil {
			return fmt.Errorf("reading repo path %s: %w", repoPath, err)
		}
		if len(entries) == 0 {
			needsClone = true
		}
	}

	if needsClone {
		if err := Clone(filepath.Dir(repoPath), cloneURL, repoPath); err != nil {
			return fmt.Errorf("cloning repo: %w", err)
		}
	}

	// verify repo path is a git repo
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err != nil {
		return fmt.Errorf("repo path %s is not a git repository: %w", repoPath, err)
	}

	// abort if there are uncommitted changes
	if err := EnsureCleanWorkingTree(repoPath); err != nil {
		return err
	}

	return nil
}

func EnsureCleanWorkingTree(repoPath string) error {
	out, err := Run(repoPath, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("checking repo status: %w", err)
	}
	if out != "" {
		return fmt.Errorf("repo at %s has uncommitted changes, aborting:\n%s", repoPath, out)
	}
	return nil
}

func FetchPRMergeRef(repoPath string, prNumber int) error {
	ref := fmt.Sprintf("pull/%d/merge", prNumber)
	_, err := Run(repoPath, "fetch", "origin", ref)
	if err != nil {
		return fmt.Errorf("fetching %s: %w (does the PR have a merge conflict?)", ref, err)
	}
	return nil
}

func CheckoutFetchHead(repoPath string) error {
	out, err := Run(repoPath, "checkout", "FETCH_HEAD")
	if err != nil {
		return err
	}
	// caller can log if needed
	_ = out
	return nil
}

func Clone(parentDir, cloneURL, targetPath string) error {
	_, err := Run(parentDir, "clone", cloneURL, targetPath)
	return err
}
