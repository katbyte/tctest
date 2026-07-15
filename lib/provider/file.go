package provider

import (
	"fmt"
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
	Path     string // full relative path: "internal/services/batch/batch_account_resource.go"
	Dir      string // directory with trailing slash: "internal/services/batch/"
	Name     string // filename: "batch_account_resource.go"
	BaseName string // filename without .go: "batch_account_resource"
	Type     FileType
}

// NewFile creates a File from a relative path and type.
func NewFile(path string, fileType FileType) File {
	lastSlash := strings.LastIndex(path, "/")
	dir := ""
	base := path
	if lastSlash >= 0 {
		dir = path[:lastSlash+1]
		base = path[lastSlash+1:]
	}

	return File{
		Path:     path,
		Dir:      dir,
		Name:     base,
		BaseName: strings.TrimSuffix(base, ".go"),
		Type:     fileType,
	}
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
	if !strings.HasSuffix(f.Path, ".go") {
		return FileTypeOther
	}

	if f.IsServicePath() {
		if f.Name == "registration.go" || f.Name == "resourceids.go" {
			return FileTypeOther
		}
	}

	if strings.HasSuffix(f.Path, "_test.go") {
		return FileTypeTest
	}

	if strings.HasPrefix(f.Path, "vendor/") {
		return FileTypeVendor
	}

	if fileRegEx.MatchString(f.Path) {
		return FileTypeResource
	}

	if f.IsServicePath() {
		return FileTypeHelper
	}

	return FileTypeOther
}
