package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/katbyte/tctest/lib/cout"
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

// EnsurePathIsRepo ensures the given path contains a git repository.
// If the directory doesn't exist or is empty, it clones from cloneURL.
// It verifies a .git directory exists. If force is true, uncommitted changes
// are discarded with git reset + clean; otherwise an error is returned.
func EnsurePathIsRepo(repoPath, cloneURL string, force bool) error {
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
		cout.Printf("  cloning <fg=208>%s</>...\n", cloneURL)
		if err := Clone(filepath.Dir(repoPath), cloneURL, repoPath); err != nil {
			return fmt.Errorf("cloning repo: %w", err)
		}
	}

	// verify repo path is a git repo
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err != nil {
		return fmt.Errorf("repo path %s is not a git repository: %w", repoPath, err)
	}

	// check for uncommitted changes
	dirty, dirtyOutput, err := IsWorkingTreeDirty(repoPath)
	if err != nil {
		return err
	}
	if dirty {
		if !force {
			return fmt.Errorf("repo at %s has uncommitted changes, aborting:\n%s", repoPath, dirtyOutput)
		}
		cout.Printf("  <yellow>resetting</> uncommitted changes...\n")
		if err := ResetAndClean(repoPath); err != nil {
			return err
		}
	}

	return nil
}

// IsWorkingTreeDirty returns true if the repo has uncommitted changes,
// along with the porcelain output describing them.
func IsWorkingTreeDirty(repoPath string) (bool, string, error) {
	out, err := Run(repoPath, "status", "--porcelain")
	if err != nil {
		return false, "", fmt.Errorf("checking repo status: %w", err)
	}
	return out != "", out, nil
}

// ResetAndClean discards all uncommitted changes (tracked and untracked).
func ResetAndClean(repoPath string) error {
	if _, err := Run(repoPath, "reset", "--hard"); err != nil {
		return fmt.Errorf("git reset --hard: %w", err)
	}
	if _, err := Run(repoPath, "clean", "-fd"); err != nil {
		return fmt.Errorf("git clean -fd: %w", err)
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

func CheckoutFetchHead(repoPath string) (string, error) {
	_, err := Run(repoPath, "checkout", "FETCH_HEAD")
	if err != nil {
		return "", err
	}
	sha, err := Run(repoPath, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("getting HEAD sha: %w", err)
	}
	return sha, nil
}

func Clone(parentDir, cloneURL, targetPath string) error {
	_, err := Run(parentDir, "clone", cloneURL, targetPath)
	return err
}
