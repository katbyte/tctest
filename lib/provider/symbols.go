package provider

import (
	"go/ast"
	"go/parser"
	"go/token"

	"github.com/katbyte/tctest/lib/clog"
)

// Symbols extracts all globally declared function/type/variable/constant names from the file.
// If exportedOnly is true, it only returns symbols starting with an uppercase letter.
func (f *File) Symbols(exportedOnly bool) []string {
	content, err := f.GetContent()
	if err != nil {
		clog.Log.Debugf("    failed to read %s for symbols: %v", f.RelPath, err)
		return nil
	}

	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, f.RelPath, content, 0)
	if err != nil {
		clog.Log.Debugf("    failed to parse %s for symbols: %v", f.RelPath, err)
		return nil
	}

	var symbols []string
	for _, decl := range parsed.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if !exportedOnly || d.Name.IsExported() {
				symbols = append(symbols, d.Name.Name)
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if !exportedOnly || s.Name.IsExported() {
						symbols = append(symbols, s.Name.Name)
					}
				case *ast.ValueSpec:
					for _, name := range s.Names {
						if !exportedOnly || name.IsExported() {
							symbols = append(symbols, name.Name)
						}
					}
				}
			}
		}
	}
	return symbols
}
