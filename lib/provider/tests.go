package provider

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"github.com/katbyte/tctest/lib/clog"
)

// ExtractTests parses Go source code (from a byte slice) and extracts names of
// acceptance tests (functions starting with "TestAcc").
// If AST parsing fails, it falls back to string regex matching.
// It also applies the provided `splitOn` and `reappend` logic.
func (f File) ExtractTests(content []byte, splitOn string, reappend bool) []string {
	var tests []string
	fset := token.NewFileSet()

	// Try parsing with AST
	parsed, parseErr := parser.ParseFile(fset, f.Path, content, 0)
	if parseErr != nil {
		clog.Log.Debugf("    failed to parse %s, falling back to string match: %v", f.Path, parseErr)
		// fallback: scan lines for "func TestAcc" if AST parsing fails
		for _, line := range strings.Split(string(content), "\n") {
			if strings.Contains(line, "func TestAcc") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					tests = append(tests, strings.Split(parts[1], "(")[0])
				}
			}
		}
	} else {
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && strings.HasPrefix(fn.Name.Name, "TestAcc") {
				clog.Log.Tracef("found test function: %s", fn.Name.Name)
				tests = append(tests, fn.Name.Name)
			}
		}
	}

	// process test names: split and optionally reappend split character
	processedTests := make([]string, 0, len(tests))
	for _, t := range tests {
		// split on `(` to make sure we just get the full function name
		testName := strings.Split(strings.Split(t, splitOn)[0], "(")[0]

		if reappend && splitOn != "" {
			testName += splitOn
		}

		processedTests = append(processedTests, testName)
	}

	return processedTests
}
