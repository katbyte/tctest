package provider

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strconv"
	"strings"

	"github.com/katbyte/tctest/lib/clog"
)

// testFunc is an acceptance test function found in a file, with the line span of its declaration (including any doc
// comment) so it can be intersected with the changed line ranges of a PR patch.
type testFunc struct {
	Name      string
	StartLine int
	EndLine   int
}

// testFuncs parses the file and returns every acceptance test function ("TestAcc" prefix) with its line span.
// If AST parsing fails it falls back to string matching, in which case line spans cover only the declaration line.
func (f *File) testFuncs() ([]testFunc, error) {
	content, err := f.GetContent()
	if err != nil {
		return nil, err
	}

	var tests []testFunc
	fset := token.NewFileSet()

	parsed, parseErr := parser.ParseFile(fset, f.Path, content, parser.ParseComments)
	if parseErr != nil {
		clog.Log.Debugf("    failed to parse %s, falling back to string match: %v", f.RelPath, parseErr)
		// fallback: scan lines for "func TestAcc" if AST parsing fails
		for i, line := range strings.Split(string(content), "\n") {
			if strings.Contains(line, "func TestAcc") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					name, _, _ := strings.Cut(parts[1], "(")
					tests = append(tests, testFunc{Name: name, StartLine: i + 1, EndLine: i + 1})
				}
			}
		}
		return tests, nil
	}

	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || !strings.HasPrefix(fn.Name.Name, "TestAcc") {
			continue
		}
		clog.Log.Tracef("found test function: %s", fn.Name.Name)

		start := fn.Pos()
		if fn.Doc != nil {
			start = fn.Doc.Pos() // a change to the doc comment still touches the test
		}
		tests = append(tests, testFunc{
			Name:      fn.Name.Name,
			StartLine: fset.Position(start).Line,
			EndLine:   fset.Position(fn.End()).Line,
		})
	}

	return tests, nil
}

// ExtractTests parses Go source code and extracts names of
// acceptance tests (functions starting with "TestAcc").
// If AST parsing fails, it falls back to string regex matching.
// It uses f.GetContent() to read the file (from cached Content or from disk).
// It also applies the provided `splitOn` and `reappend` logic.
func (f *File) ExtractTests(splitOn string, reappend bool) ([]string, error) {
	tests, err := f.testFuncs()
	if err != nil {
		return nil, err
	}

	// process test names: split and optionally reappend split character
	processedTests := make([]string, 0, len(tests))
	for _, t := range tests {
		// split on `(` to make sure we just get the full function name
		beforeSplit, _, _ := strings.Cut(t.Name, splitOn)
		testName, _, _ := strings.Cut(beforeSplit, "(")

		if reappend && splitOn != "" {
			testName += splitOn
		}

		processedTests = append(processedTests, testName)
	}

	return processedTests, nil
}

// hunkHeaderRegex matches unified diff hunk headers like "@@ -12,7 +12,9 @@" and captures the new-side start line and
// (optional) line count.
var hunkHeaderRegex = regexp.MustCompile(`(?m)^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

// lineRange is an inclusive range of line numbers on the new side of a diff.
type lineRange struct {
	start, end int
}

// changedLineRanges parses the file's stored patch into the new-side line ranges its hunks cover.
func (f *File) changedLineRanges() []lineRange {
	matches := hunkHeaderRegex.FindAllStringSubmatch(f.patch, -1)
	ranges := make([]lineRange, 0, len(matches))
	for _, m := range matches {
		start, _ := strconv.Atoi(m[1])
		count := 1
		if m[2] != "" {
			count, _ = strconv.Atoi(m[2])
		}
		if count < 1 {
			count = 1 // a pure deletion (+N,0) still marks position N as touched
		}
		ranges = append(ranges, lineRange{start: start, end: start + count - 1})
	}
	return ranges
}

// DiscoverTests extracts a file's test names for build triggering. With individual set, a directly changed test file
// (which carries its PR patch) yields only the full names of the test functions the PR modifies; everything else —
// individual off, no patch (e.g. a file discovered via a sibling resource), or a diff touching only non-test code —
// falls back to ExtractTests' split-prefix behaviour over the whole file.
func (f *File) DiscoverTests(splitOn string, reappend, individual bool) ([]string, error) {
	if individual {
		tests, ok, err := f.ExtractChangedTests()
		if err != nil {
			return nil, err
		}
		if ok && len(tests) > 0 {
			return tests, nil
		}
		if ok {
			// the diff only touched non-test code in the file (e.g. a shared helper), so any of its tests could be
			// affected — better to fall back to whole-file discovery than silently discover nothing
			clog.Log.Debugf("    %s: no test functions overlap the PR diff, falling back to whole-file discovery", f.RelPath)
		}
	}
	return f.ExtractTests(splitOn, reappend)
}

// ExtractChangedTests returns the full (unsplit) names of the acceptance test functions whose declarations overlap
// the file's PR patch — i.e. only the tests actually modified, not every test in the file. It returns ok=false when
// the file has no stored patch (it wasn't directly changed in the PR, e.g. discovered via a sibling resource file),
// in which case the caller should fall back to ExtractTests' prefix behaviour.
func (f *File) ExtractChangedTests() (tests []string, ok bool, err error) {
	changed := f.changedLineRanges()
	if len(changed) == 0 {
		return nil, false, nil
	}

	funcs, err := f.testFuncs()
	if err != nil {
		return nil, false, err
	}

	for _, fn := range funcs {
		for _, r := range changed {
			if fn.StartLine <= r.end && r.start <= fn.EndLine {
				tests = append(tests, fn.Name)
				break
			}
		}
	}

	return tests, true, nil
}
