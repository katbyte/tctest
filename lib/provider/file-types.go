package provider

import (
	"fmt"
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

// Classifier sets the file type for a File.
// It can be overridden at program start to provide custom logic.
var Classifier = DefaultClassifier

// DefaultClassifier provides the base classification logic.
func DefaultClassifier(f *File) FileType {
	if !strings.HasSuffix(f.RelPath, ".go") {
		return FileTypeOther
	}

	if f.InServicePackage() {
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

	if f.InServicePackage() {
		if strings.HasSuffix(f.Name, "_resource.go") || strings.HasSuffix(f.Name, "_data_source.go") {
			return FileTypeResource
		}
		return FileTypeHelper
	}

	return FileTypeOther
}

// Classify determines the FileType for this file using the global Classifier and sets f.Type.
func (f *File) Classify() FileType {
	f.Type = Classifier(f)
	return f.Type
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

const (
	FileColourOther    = "<darkGray>"
	FileColourResource = "<fg=36>"
	FileColourHelper   = "<fg=117>"
	FileColourTest     = "<fg=28>"
	FileColourUnitTest = "<darkGray>"
	FileColourVendor   = "<fg=177>"
	FileColourDerived  = "<fg=36>"
	FileColourSkipped  = "<red>"
)

// TextColour returns the cout colour tag for this file type.
func (f File) TextColour() string {
	switch f.Type {
	case FileTypeOther:
		return FileColourOther
	case FileTypeResource:
		return FileColourResource
	case FileTypeHelper:
		return FileColourHelper
	case FileTypeTest:
		return FileColourTest
	case FileTypeUnitTest:
		return FileColourUnitTest
	case FileTypeVendor:
		return FileColourVendor
	default:
		return FileColourOther
	}
}

// ColouredOutput returns the formatted dir + coloured base for cout output.
// Example: "<darkGray>internal/services/batch/</><fg=36>batch_account_resource.go</>"
func (f File) ColouredOutput() string {
	return fmt.Sprintf("<darkGray>%s</>%s%s</>", f.Dir, f.TextColour(), f.Name)
}
