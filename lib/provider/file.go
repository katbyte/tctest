package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// FileType classifies a Go file within a Terraform provider repository.
type FileType int

const (
	FileTypeOther    FileType = iota // outside service directories
	FileTypeResource                 // matches fileregex (resource or data source at service root)
	FileTypeHelper                   // in service dir but doesn't match fileregex (e.g. parse/, validate/, client/)
	FileTypeTest                     // _test.go with TestAcc functions
	FileTypeUnitTest                 // _test.go without TestAcc functions
	FileTypeVendor                   // under vendor/
)

// File represents a Go source file in a Terraform provider repository.
type File struct {
	RelPath  string // full relative path: "internal/services/batch/batch_account_resource.go"
	Path     string // absolute local path (or empty if using Content buffer)
	Dir      string // directory with trailing slash: "internal/services/batch/"
	Name     string // filename: "batch_account_resource.go"
	BaseName string // filename without .go: "batch_account_resource"
	Type     FileType
	Content  []byte // optional file content for self-reading methods
}

// NewFileWithPath creates a File from a relative path and a local repository root.
func NewFileWithPath(relPath, repoPath string) File {
	f := newFile(relPath)
	f.Path = filepath.Join(repoPath, relPath)
	return f
}

// NewFileWithContent creates a File with pre-loaded content (e.g. downloaded from GitHub).
func NewFileWithContent(relPath string, content []byte) File {
	f := newFile(relPath)
	// no local disk path — content is already in memory
	f.Content = content
	return f
}

// newFile creates a File with only the path-derived fields populated.
func newFile(relPath string) File {
	lastSlash := strings.LastIndex(relPath, "/")
	dir := ""
	base := relPath
	if lastSlash >= 0 {
		dir = relPath[:lastSlash+1]
		base = relPath[lastSlash+1:]
	}

	return File{
		RelPath:  relPath,
		Dir:      dir,
		Name:     base,
		BaseName: strings.TrimSuffix(base, ".go"),
	}
}

// Classify determines the FileType for this file using the given regex and sets f.Type.
// Note: for test files, this always returns FileTypeTest. Use ClassifyTestFile
// to distinguish between acceptance tests and unit tests (requires file contents).
func (f *File) Classify(fileRegEx *regexp.Regexp) FileType {
	if !strings.HasSuffix(f.RelPath, ".go") {
		f.Type = FileTypeOther
		return f.Type
	}

	if f.IsServicePath() {
		if f.Name == "registration.go" || f.Name == "resourceids.go" {
			f.Type = FileTypeOther
			return f.Type
		}
	}

	if strings.HasSuffix(f.RelPath, "_test.go") {
		// unit test vs acceptance test determination is handled later during ast parsing
		f.Type = FileTypeTest
		return f.Type
	}
	if strings.HasPrefix(f.RelPath, "vendor/") {
		f.Type = FileTypeVendor
		return f.Type
	}

	if fileRegEx != nil && fileRegEx.MatchString(f.RelPath) {
		f.Type = FileTypeResource
		return f.Type
	}

	if f.IsServicePath() {
		f.Type = FileTypeHelper
		return f.Type
	}

	f.Type = FileTypeOther
	return f.Type
}

// GetContent returns the file's content. It uses the cached Content buffer if
// available, otherwise reads from the absolute Path on disk and caches the result.
func (f *File) GetContent() ([]byte, error) {
	if len(f.Content) > 0 {
		return f.Content, nil
	}

	if f.Path == "" {
		return nil, fmt.Errorf("file %s: no content and no local path", f.RelPath)
	}

	content, err := os.ReadFile(f.Path) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", f.Path, err)
	}

	f.Content = content
	return content, nil
}

// ResourcePrefix returns the prefix used for test file discovery.
// For "batch_account_resource.go" → "batch_account".
// For "batch_account_data_source.go" → "batch_account_data_source".
func (f File) ResourcePrefix() string {
	return strings.TrimSuffix(f.BaseName, "_resource")
}

// TypeLabel returns the display label for this file type (e.g. "[RESOURCE]", "[HELPER]").
func (f File) TypeLabel() string {
	switch f.Type {
	case FileTypeOther:
		return "[OTHER]"
	case FileTypeResource:
		return "[RESOURCE]"
	case FileTypeHelper:
		return "[HELPER]"
	case FileTypeTest:
		return "[TEST]"
	case FileTypeUnitTest:
		return "[UNIT]"
	case FileTypeVendor:
		return "[VENDOR]"
	default:
		return "[OTHER]"
	}
}

// Colour returns the cout colour tag for this file type.
func (f File) Colour() string {
	switch f.Type {
	case FileTypeOther:
		return "<darkGray>"
	case FileTypeResource:
		return "<fg=36>"
	case FileTypeHelper:
		return "<fg=117>"
	case FileTypeTest:
		return "<fg=28>"
	case FileTypeUnitTest:
		return "<darkGray>"
	case FileTypeVendor:
		return "<fg=177>"
	default:
		return "<darkGray>"
	}
}

// ColouredOutput returns the formatted dir + coloured base for cout output.
// Example: "<darkGray>internal/services/batch/</><fg=36>batch_account_resource.go</>"
func (f File) ColouredOutput() string {
	return fmt.Sprintf("<darkGray>%s</>%s%s</>", f.Dir, f.Colour(), f.Name)
}
