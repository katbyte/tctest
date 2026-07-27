package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ServiceDirPrefixes lists the path segments used by different providers to
// organise service packages. Azure uses "internal/services/", AWS uses
// "internal/service/". All code that needs to detect or enumerate service
// directories should iterate over this slice.
var ServiceDirPrefixes = []string{"internal/services", "internal/service"}

// File represents a Go source file in a Terraform provider repository.
type File struct {
	RelPath      string // full relative path: "internal/services/batch/batch_account_resource.go"
	Path         string // absolute local path (or empty if using Content buffer)
	Dir          string // directory with trailing slash: "internal/services/batch/"
	Name         string // filename: "batch_account_resource.go"
	BaseName     string // filename without .go: "batch_account_resource"
	Service      string // the extracted service name from the path, e.g. "batch"
	Type         FileType
	DiscoveredBy []string // e.g. CHANGED, DERIVED, TRACED, VENDOR
	content      []byte   // optional file content for self-reading methods
}

// NewFileWithPath creates a File from a relative path and a local repository root.
func NewFileWithPath(relPath, repoPath string) File {
	f := NewFile(relPath)
	f.Path = filepath.Join(repoPath, relPath)
	f.Classify()
	return f
}

// NewFile creates a File with only the path-derived fields populated.
// Use this for files in a remote environment where a local disk path isn't applicable.
func NewFile(relPath string) File {
	lastSlash := strings.LastIndex(relPath, "/")
	dir := ""
	base := relPath
	if lastSlash >= 0 {
		dir = relPath[:lastSlash+1]
		base = relPath[lastSlash+1:]
	}

	f := File{
		RelPath:  relPath,
		Dir:      dir,
		Name:     base,
		BaseName: strings.TrimSuffix(base, ".go"),
	}

	for _, prefix := range ServiceDirPrefixes {
		sep := "/" + filepath.Base(prefix) + "/"
		parts := strings.Split(f.RelPath, sep)
		if len(parts) == 2 {
			f.Service = strings.Split(parts[1], "/")[0]
			break
		}
	}

	f.Classify()
	return f
}

// GetContent returns the file's content. It uses the cached Content buffer if
// available, otherwise reads from the absolute Path on disk and caches the result.
func (f *File) GetContent() ([]byte, error) {
	if len(f.content) > 0 {
		return f.content, nil
	}

	if f.Path == "" {
		return nil, fmt.Errorf("file %s: no content and no local path", f.RelPath)
	}

	content, err := os.ReadFile(f.Path) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", f.Path, err)
	}

	f.content = content
	return content, nil
}

// SetContent sets the file's internal content buffer and re-runs classification.
func (f *File) SetContent(content []byte) {
	f.content = content
	f.Classify()
}

// InServicePackage returns true if the path is within a service directory.
func (f File) InServicePackage() bool {
	for _, prefix := range ServiceDirPrefixes {
		if strings.Contains(f.RelPath, "/"+filepath.Base(prefix)+"/") {
			return true
		}
	}
	return false
}

// ResourcePrefix returns the prefix used for test file discovery.
// For "batch_account_resource.go" → "batch_account".
// For "batch_account_data_source.go" → "batch_account_data_source".
func (f File) ResourcePrefix() string {
	return strings.TrimSuffix(f.BaseName, "_resource")
}

// AddDiscovery adds a discovery source label if it isn't already present.
func (f *File) AddDiscovery(source string) {
	for _, s := range f.DiscoveredBy {
		if s == source {
			return
		}
	}
	f.DiscoveredBy = append(f.DiscoveredBy, source)
}
