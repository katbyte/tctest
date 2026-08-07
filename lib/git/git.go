// Package git runs local git commands for cloning repos, fetching PR merge refs, and checking out commits.
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
func IsWorkingTreeDirty(repoPath string) (dirty bool, output string, err error) {
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

// GetCurrentRef returns the current branch name, tag name, or commit SHA (in that order of preference).
func GetCurrentRef(repoPath string) (string, error) {
	if branch, err := Run(repoPath, "symbolic-ref", "--short", "HEAD"); err == nil {
		return branch, nil
	}
	if tag, err := Run(repoPath, "describe", "--exact-match", "--tags", "HEAD"); err == nil {
		return tag, nil
	}
	sha, err := Run(repoPath, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("getting current ref: %w", err)
	}
	return sha, nil
}

// CheckoutRef checks out the given branch name or commit SHA.
func CheckoutRef(repoPath, ref string) error {
	if _, err := Run(repoPath, "checkout", ref); err != nil {
		return fmt.Errorf("git checkout %s: %w", ref, err)
	}
	return nil
}

// IsRepoForRemote returns true if the git repo at repoPath has a remote URL
// that refers to the same GitHub repository as cloneURL. It normalises both
// URLs to "host/owner/repo" before comparing so that SSH and HTTPS forms match.
func IsRepoForRemote(repoPath, cloneURL string) bool {
	out, err := Run(repoPath, "remote", "get-url", "origin")
	if err != nil {
		return false
	}
	return normaliseGitURL(out) == normaliseGitURL(cloneURL)
}

// normaliseGitURL strips protocol/host boilerplate and a trailing .git so that
// SSH ("git@github.com:owner/repo.git") and HTTPS ("https://github.com/owner/repo")
// forms of the same URL compare equal.
func normaliseGitURL(u string) string {
	u = strings.ToLower(strings.TrimSpace(u))
	u = strings.TrimSuffix(u, ".git")
	// SSH: git@github.com:owner/repo -> github.com/owner/repo
	if after, ok := strings.CutPrefix(u, "git@"); ok {
		return strings.Replace(after, ":", "/", 1)
	}
	// HTTPS: https://github.com/owner/repo -> github.com/owner/repo
	for _, prefix := range []string{"https://", "http://"} {
		if after, ok := strings.CutPrefix(u, prefix); ok {
			return after
		}
	}
	return u
}
