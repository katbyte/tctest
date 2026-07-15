package provider

import (
	"fmt"
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

// NewFileWithPath creates a File from a relative path, an absolute repository path, and a type.
func NewFileWithPath(relPath, repoPath string, fileType FileType) File {
	lastSlash := strings.LastIndex(relPath, "/")
	dir := ""
	base := relPath
	if lastSlash >= 0 {
		dir = relPath[:lastSlash+1]
		base = relPath[lastSlash+1:]
	}

	return File{
		RelPath:  relPath,
		Path:     filepath.Join(repoPath, relPath),
		Dir:      dir,
		Name:     base,
		BaseName: strings.TrimSuffix(base, ".go"),
		Type:     fileType,
		Content:  nil,
	}
}

// NewFileWithContent creates a File and initialises its Content.
func NewFileWithContent(relPath string, fileType FileType, content []byte) File {
	f := NewFileWithPath(relPath, "", fileType)
	f.Path = "" // no local disk path
	f.Content = content
	return f
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

// Classify determines the FileType for this file using the given regex.
// Note: for test files, this always returns FileTypeTest. Use ClassifyTestFile
// to distinguish between acceptance tests and unit tests (requires file contents).
func (f File) Classify(fileRegEx *regexp.Regexp) FileType {
	if !strings.HasSuffix(f.RelPath, ".go") {
		return FileTypeOther
	}

	if f.IsServicePath() {
		if f.Name == "registration.go" || f.Name == "resourceids.go" {
			return FileTypeOther
		}
	}

	if strings.HasSuffix(f.RelPath, "_test.go") {
		// unit test vs acceptance test determination is handled later during ast parsing
		return FileTypeTest
	}
	if strings.HasPrefix(f.RelPath, "vendor/") {
		return FileTypeVendor
	}

	if fileRegEx.MatchString(f.RelPath) {
		return FileTypeResource
	}

	if f.IsServicePath() {
		return FileTypeHelper
	}

	return FileTypeOther
}
