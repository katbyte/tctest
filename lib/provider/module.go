package provider

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GetModulePath reads go.mod in the repo and returns the module import path.
func GetModulePath(repoPath string) (string, error) {
	data, err := os.ReadFile(filepath.Join(repoPath, "go.mod")) //nolint:gosec // path is from user-provided --local-repo-path flag
	if err != nil {
		return "", fmt.Errorf("reading go.mod: %w", err)
	}

	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(after), nil
		}
	}

	return "", errors.New("module directive not found in go.mod")
}
