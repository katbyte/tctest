package provider

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
)

// Symbols parses the Go file and returns symbol names (functions, types, variables, constants).
// repoPath is the root of the repository; the full path is constructed as repoPath/f.Path.
// If exportedOnly is true, only exported (uppercase) symbols are returned.
// If exportedOnly is false, all symbols are returned (for same-package tracing).
func (f File) Symbols(repoPath string, exportedOnly bool) []string {
	fullPath := filepath.Join(repoPath, f.Path)

	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, fullPath, nil, 0)
	if err != nil {
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
