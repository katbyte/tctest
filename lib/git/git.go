package git

import (
	"context"
	"fmt"
	"os/exec"
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
