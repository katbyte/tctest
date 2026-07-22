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

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}

	return "", errors.New("module directive not found in go.mod")
}
