package cli

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/google/go-github/v45/github"
	"github.com/katbyte/tctest/lib/clog"
	"github.com/katbyte/tctest/lib/cout"
	"github.com/katbyte/tctest/lib/git"
	"github.com/katbyte/tctest/lib/provider"
)

type AstDiscoveryContext struct {
	RepoPath   string
	ModulePath string
	Config     DiscoveryConfig

	// File tracking
	TestFiles map[string]*provider.File

	// Accumulators for PrintDiscoveredFiles
	ChangedFileLines []string
}

func NewAstDiscoveryContext(repoPath, modulePath string, cfg DiscoveryConfig) *AstDiscoveryContext {
	return &AstDiscoveryContext{
		RepoPath:   repoPath,
		ModulePath: modulePath,
		Config:     cfg,

		TestFiles: make(map[string]*provider.File),

		ChangedFileLines: make([]string, 0),
	}
}

// PrTestsFromAst performs test discovery using a local git clone of the repository.
// When cfg.LocalMode is AST, this is called instead of PrTestsFromAPI (the HTTP-based path).
//
// It fetches the PR merge ref, checks out the code, and uses Go AST to discover
// affected tests — including tracing imports from helper/validation files back to
// resource files to find their tests.
func (ghr GithubRepo) PrTestsFromAst(pri int, cfg DiscoveryConfig) (*map[string][]string, error) {
	repoPath, err := filepath.Abs(cfg.LocalRepoPath)
	if err != nil {
		return nil, fmt.Errorf("resolving repo path: %w", err)
	}

	// ensure repo path is a clean git clone (cloning if needed)
	cout.Printf("  local AST detection: <fg=208>%s</> trace depth <yellow>%d</>\n", repoPath, cfg.LocalTraceDepth)
	if err := git.EnsurePathIsRepo(repoPath, ghr.CloneURL()); err != nil {
		return nil, err
	}

	// fetch PR merge ref and checkout
	cout.Printf("  fetching PR <cyan>#%d</> merge ref...\n", pri)
	if err := ghr.CheckoutPR(repoPath, pri); err != nil {
		return nil, err
	}

	// check PR state via GitHub API
	client, ctx := ghr.NewClient()
	clog.Log.Debugf("fetching data for PR %s/%s/#%d...", ghr.Owner, ghr.Name, pri)
	pr, _, err := client.PullRequests.Get(ctx, ghr.Owner, ghr.Name, pri)
	if err != nil {
		return nil, err
	}
	if pr.GetState() == "closed" {
		return nil, errors.New("cannot start build for a closed pr")
	}

	// get module path from go.mod for import tracing
	modulePath, err := provider.GetModulePath(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read module path: %w", err)
	}
	clog.Log.Debugf("  module path: %s", modulePath)

	dc := NewAstDiscoveryContext(repoPath, modulePath, cfg)

	// print config
	cout.Verbosef("  file regex: <darkGray>%s</>\n", cfg.FileRegEx.String())
	cout.Verbosef("  acctest file suffix patterns: <darkGray>%s</>\n", cfg.AccTestFileSuffixRegexStrings())

	// fetch and categorise
	resourcePrefixesByPackage, helperFiles, vendorFiles, err := dc.CollectChangedFiles(ghr, pri)
	if err != nil {
		return nil, err
	}

	// trace files
	dc.DiscoverSiblingTests(resourcePrefixesByPackage)
	dc.TraceHelperFiles(helperFiles)
	dc.TraceVendorFiles(vendorFiles)

	// summarize results
	dc.PrintDiscoveredFiles()

	// parse tests
	tests, err := dc.ParseTestsConcurrently()
	if err != nil {
		return nil, err
	}

	clog.Log.Debugf("  FOUND %d services", len(tests))
	return &tests, nil
}

// --- Local test file discovery ---

// findLocalTestFiles walks a local directory and returns test files matching
// the resource prefix + suffix regex patterns. Same matching logic as the
// GitHub Contents API path, but reads from the local filesystem (no 1000-file cap).
func (dc *AstDiscoveryContext) findLocalTestFiles(relativeDir string, resourcePrefixes []string) ([]provider.File, error) {
	localDir := filepath.Join(dc.RepoPath, relativeDir)

	entries, err := os.ReadDir(localDir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", localDir, err)
	}

	var testFiles []provider.File
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		pf := provider.NewFileWithPath(filepath.Join(relativeDir, entry.Name()), dc.RepoPath)
		if pf.Type != provider.FileTypeTest && pf.Type != provider.FileTypeUnitTest {
			continue
		}

		matched := false
		for _, prefix := range resourcePrefixes {
			if !strings.HasPrefix(pf.BaseName, prefix) {
				continue
			}
			remainder := pf.BaseName[len(prefix):]

			for _, re := range dc.Config.AccTestFileSuffixRegexes {
				if re.MatchString(remainder) {
					matched = true
					break
				}
			}

			if matched {
				break
			}
		}

		if matched {
			testFiles = append(testFiles, pf)
		}
	}

	return testFiles, nil
}

// --- Import tracing ---

// traceImportsToResourceFiles performs BFS import tracing from helper files to find
// affected resource files within the same service boundary.
//
// For each helper file (e.g., internal/services/network/parse/helper.go), it:
//  1. Determines the helper's Go package import path
//  2. Walks the parent service directory
//  3. Full-parses each .go file and checks for SelectorExpr usage of specific exported symbols
//  4. If a file uses a changed symbol AND matches the fileregex, it's an affected resource
//  5. If it uses a changed symbol but doesn't match fileregex, it's queued for the next depth level
//
// Returns map[dir][]string (same format as resourcePrefixesByPackage in GetPullRequestTestFiles).
func (dc *AstDiscoveryContext) traceImportsToResourceFiles(helperFiles []provider.File, pkgSymbols map[string]map[string]bool) map[string][]string {
	result := map[string][]string{}

	// collect unique packages of helper files
	currentLevel := map[string]string{} // package import path -> service directory
	visited := map[string]bool{}

	for _, pf := range helperFiles {
		f := pf.RelPath
		dir := filepath.ToSlash(filepath.Dir(f))
		pkgPath := dc.ModulePath + "/" + dir

		// extract service directory from file path
		// e.g., "internal/services/network/parse/helper.go" -> "internal/services/network"
		serviceDir := ""
		for _, prefix := range []string{"internal/services/", "internal/service/"} {
			idx := strings.Index(f, prefix)
			if idx < 0 {
				continue
			}
			rest := f[idx+len(prefix):]
			parts := strings.SplitN(rest, "/", 2)
			if len(parts) >= 1 && parts[0] != "" {
				serviceDir = f[:idx+len(prefix)] + parts[0]
			}
		}
		if serviceDir == "" {
			clog.Log.Debugf("    skipping %s: not in a service directory", f)
			continue
		}
		if !visited[pkgPath] {
			currentLevel[pkgPath] = serviceDir
			visited[pkgPath] = true
			clog.Log.Debugf("    tracing package: %s (service dir: %s)", pkgPath, serviceDir)
		}
	}

	// BFS: at each depth level, find files that import the current set of packages
	for depth := 0; depth < dc.Config.LocalTraceDepth && len(currentLevel) > 0; depth++ {
		nextLevel := map[string]string{}

		for pkgPath, serviceDir := range currentLevel {
			localServiceDir := filepath.Join(dc.RepoPath, serviceDir)
			symbols := pkgSymbols[pkgPath] // may be nil if no exported symbols tracked

			err := filepath.WalkDir(localServiceDir, func(path string, d os.DirEntry, walkErr error) error {
				if walkErr != nil {
					//nolint:nilerr // WalkDir: skip files with errors, continue walking
					return nil
				}
				if d.IsDir() {
					return nil // keep walking into subdirs
				}
				if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
					return nil
				}

				// get path relative to repo root
				relPath, relErr := filepath.Rel(dc.RepoPath, path)
				if relErr != nil {
					//nolint:nilerr // filepath.Rel failure is non-fatal, skip this file
					return nil
				}
				relPath = filepath.ToSlash(relPath)

				// full parse to check both imports and symbol usage
				fset := token.NewFileSet()
				parsed, parseErr := parser.ParseFile(fset, path, nil, 0)
				if parseErr != nil {
					//nolint:nilerr // parse failure is non-fatal, skip this file
					return nil
				}

				// find the import alias for the target package
				importAlias := ""
				for _, imp := range parsed.Imports {
					importPath := strings.Trim(imp.Path.Value, `"`)
					if importPath != pkgPath {
						continue
					}
					if imp.Name != nil {
						importAlias = imp.Name.Name // explicit alias
					} else {
						// default alias is last path segment
						parts := strings.Split(importPath, "/")
						importAlias = parts[len(parts)-1]
					}
					break
				}
				if importAlias == "" {
					return nil // doesn't import the target package
				}

				// if we have no symbol info, fall back to package-level matching
				if len(symbols) == 0 {
					if dc.Config.FileRegEx.MatchString(relPath) {
						dir := filepath.ToSlash(filepath.Dir(relPath))
						result[dir] = append(result[dir], relPath)
						clog.Log.Debugf("    traced: %s imports %s (depth %d, package-level)", relPath, pkgPath, depth+1)
					} else {
						helperPkg := dc.ModulePath + "/" + filepath.ToSlash(filepath.Dir(relPath))
						if !visited[helperPkg] {
							nextLevel[helperPkg] = serviceDir
							visited[helperPkg] = true
						}
					}
					return nil
				}

				// walk the AST looking for SelectorExpr: alias.Symbol
				usesSymbol := false
				var usedSymbols []string
				ast.Inspect(parsed, func(n ast.Node) bool {
					sel, ok := n.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					ident, ok := sel.X.(*ast.Ident)
					if !ok || ident.Name != importAlias {
						return true
					}
					if symbols[sel.Sel.Name] {
						usesSymbol = true
						usedSymbols = append(usedSymbols, sel.Sel.Name)
					}
					return true
				})

				if !usesSymbol {
					clog.Log.Debugf("    skipped: %s imports %s but doesn't use changed symbols", relPath, pkgPath)
					return nil
				}

				// this file uses a changed symbol
				if dc.Config.FileRegEx.MatchString(relPath) {
					// it's a resource file — add to results
					dir := filepath.ToSlash(filepath.Dir(relPath))
					result[dir] = append(result[dir], relPath)
					clog.Log.Debugf("    traced: %s uses %v from %s (depth %d)", relPath, usedSymbols, pkgPath, depth+1)
				} else {
					// it's another helper — queue for next depth
					helperPkg := dc.ModulePath + "/" + filepath.ToSlash(filepath.Dir(relPath))
					if !visited[helperPkg] {
						nextLevel[helperPkg] = serviceDir
						visited[helperPkg] = true
						clog.Log.Debugf("    intermediate: %s uses %v from %s, queuing for depth %d", relPath, usedSymbols, pkgPath, depth+2)
					}
				}

				return nil
			})
			if err != nil {
				clog.Log.Debugf("    error walking %s: %v", localServiceDir, err)
			}
		}

		currentLevel = nextLevel
	}

	return result
}

func (dc *AstDiscoveryContext) SortedTestFiles() []*provider.File {
	var files []*provider.File
	for _, pf := range dc.TestFiles {
		files = append(files, pf)
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].RelPath < files[j].RelPath
	})
	return files
}

func (dc *AstDiscoveryContext) AddTestFile(pf provider.File, source string) {
	existing, ok := dc.TestFiles[pf.RelPath]
	if !ok {
		existing = &pf
		dc.TestFiles[pf.RelPath] = existing
	}
	existing.AddDiscovery(source)
}

func (dc *AstDiscoveryContext) CollectChangedFiles(ghr GithubRepo, pri int) (map[string][]string, []provider.File, []provider.File, error) {
	resourcePrefixesByPackage := map[string][]string{}
	var helperFiles []provider.File
	var vendorFiles []provider.File

	err := ghr.ListAllPullRequestFiles(pri, func(files []*github.CommitFile, _ *github.Response) error {
		for _, f := range files {
			if f.Filename == nil {
				continue
			}
			pf := provider.NewFileWithPath(f.GetFilename(), dc.RepoPath)
			clog.Log.Debugf("    %v (%s)", pf.RelPath, f.GetStatus())

			if !strings.HasSuffix(pf.RelPath, ".go") {
				clog.Log.Debugf("    skipping non go file: %s", pf.RelPath)
				continue
			}
			if f.GetStatus() == "removed" {
				clog.Log.Debugf("    skipping removed file: %s", pf.RelPath)
				continue
			}

			switch pf.Type {
			case provider.FileTypeOther:
				dc.ChangedFileLines = append(dc.ChangedFileLines, fmt.Sprintf("    %s <darkGray>%s</>\n", pf.ColouredFileName(), pf.TypeLabel()))

			case provider.FileTypeTest:

				dc.AddTestFile(pf, "CHANGED")
				dc.ChangedFileLines = append(dc.ChangedFileLines, fmt.Sprintf("    %s <darkGray>[TEST]</>\n", pf.ColouredFileName()))

			case provider.FileTypeUnitTest:
				clog.Log.Debugf("    %s: no TestAcc functions, skipping", pf.RelPath)
				dc.ChangedFileLines = append(dc.ChangedFileLines, fmt.Sprintf("    %s <darkGray>[UNIT]</>\n", pf.ColouredFileName()))

			case provider.FileTypeResource:
				resourcePrefixesByPackage[path.Dir(pf.RelPath)] = append(resourcePrefixesByPackage[path.Dir(pf.RelPath)], pf.ResourcePrefix())
				dc.ChangedFileLines = append(dc.ChangedFileLines, fmt.Sprintf("    %s <darkGray>[RESOURCE]</>\n", pf.ColouredFileName()))

			case provider.FileTypeHelper:
				helperFiles = append(helperFiles, pf)
				dc.ChangedFileLines = append(dc.ChangedFileLines, fmt.Sprintf("    %s <darkGray>[HELPER]</>\n", pf.ColouredFileName()))

			case provider.FileTypeVendor:
				vendorFiles = append(vendorFiles, pf)
				dc.ChangedFileLines = append(dc.ChangedFileLines, fmt.Sprintf("    %s <darkGray>[VENDOR]</>\n", pf.ColouredFileName()))
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get PR files: %w", err)
	}

	cout.Printf("  changed files: <yellow>%d</>\n", len(dc.ChangedFileLines))
	showChangedFiles := dc.Config.CollapseFilesAfter == 0 || len(dc.ChangedFileLines) <= dc.Config.CollapseFilesAfter
	for _, line := range dc.ChangedFileLines {
		if showChangedFiles {
			cout.Printf("%s", line)
		} else {
			cout.Verbosef("%s", line)
		}
	}
	if !showChangedFiles && cout.Level < cout.VerbosityVerbose {
		cout.Printf("    <yellow>%d</> <fg=208>exceeds display limit of</> <yellow>%d</><darkGray>, use -v or --collapse-files-after 0 to see all</>\n", len(dc.ChangedFileLines), dc.Config.CollapseFilesAfter)
	}

	return resourcePrefixesByPackage, helperFiles, vendorFiles, nil
}

func (dc *AstDiscoveryContext) DiscoverSiblingTests(resourcePrefixesByPackage map[string][]string) {
	for dir, prefixes := range resourcePrefixesByPackage {
		discovered, err := dc.findLocalTestFiles(dir, prefixes)
		if err != nil {
			clog.Log.Debugf("  failed to find test files in %s: %v", dir, err)
			continue
		}
		for _, pf := range discovered {
			dc.AddTestFile(pf, "DERIVED")
		}
	}
}

func (dc *AstDiscoveryContext) TraceHelperFiles(helperFiles []provider.File) {
	if len(helperFiles) == 0 || dc.Config.LocalTraceDepth == 0 {
		return
	}

	var crossPkgHelpers []provider.File
	samePkgHelpers := map[string][]provider.File{}

	for _, pf := range helperFiles {
		f := pf.RelPath
		dir := filepath.ToSlash(filepath.Dir(f))
		isSamePkg := false
		if entries, err := os.ReadDir(filepath.Join(dc.RepoPath, dir)); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
					if dc.Config.FileRegEx.MatchString(filepath.ToSlash(filepath.Join(dir, entry.Name()))) {
						isSamePkg = true
						break
					}
				}
			}
		}

		if isSamePkg {
			samePkgHelpers[dir] = append(samePkgHelpers[dir], pf)
		} else {
			crossPkgHelpers = append(crossPkgHelpers, pf)
		}
	}

	samePkgHelperCount := 0
	for _, h := range samePkgHelpers {
		samePkgHelperCount += len(h)
	}
	samePkgTracedFiles := map[string]bool{}
	allHelperTraced := map[string][]provider.File{}

	for dir, helpers := range samePkgHelpers {
		symbols := map[string]bool{}
		for _, pf := range helpers {
			for _, s := range pf.Symbols(false) {
				symbols[s] = true
			}
			clog.Log.Debugf("    same-pkg helper %s symbols: %v", pf.RelPath, symbols)
		}
		if len(symbols) == 0 {
			for _, pf := range helpers {
				cout.Verbosef("    <darkGray>%s</><white;op=bold>%s</> → <darkGray>no symbols found</>\n", pf.Dir, pf.Name)
			}
			continue
		}

		localDir := filepath.Join(dc.RepoPath, dir)
		entries, err := os.ReadDir(localDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			relPath := filepath.ToSlash(filepath.Join(dir, entry.Name()))
			if !dc.Config.FileRegEx.MatchString(relPath) {
				continue
			}

			fset := token.NewFileSet()
			parsed, parseErr := parser.ParseFile(fset, filepath.Join(localDir, entry.Name()), nil, 0)
			if parseErr != nil {
				continue
			}

			var usedSymbols []string
			ast.Inspect(parsed, func(n ast.Node) bool {
				ident, ok := n.(*ast.Ident)
				if !ok {
					return true
				}
				if symbols[ident.Name] {
					usedSymbols = append(usedSymbols, ident.Name)
				}
				return true
			})

			if len(usedSymbols) > 0 {
				tracedFile := provider.NewFileWithPath(relPath, dc.RepoPath)
				clog.Log.Debugf("    same-pkg traced: %s uses %v", relPath, usedSymbols)

				resourcePrefixes := []string{tracedFile.ResourcePrefix()}
				for _, f := range helpers {
					allHelperTraced[f.RelPath] = append(allHelperTraced[f.RelPath], tracedFile)
				}
				samePkgTracedFiles[relPath] = true

				discovered, err := dc.findLocalTestFiles(dir, resourcePrefixes)
				if err != nil {
					continue
				}
				for _, pf := range discovered {
					dc.AddTestFile(pf, "TRACED")
				}
			}
		}
	}

	if samePkgHelperCount > 0 {
		if cout.Level >= cout.VerbosityVerbose {
			cout.Printf("  tracing symbols from <yellow>%d</> same-package helper file(s)...\n", samePkgHelperCount)
		} else {
			cout.Printf("  tracing symbols from <yellow>%d</> same-package helper file(s)... <cyan>%d</> resource file(s)\n", samePkgHelperCount, len(samePkgTracedFiles))
		}
	}

	for _, helpers := range samePkgHelpers {
		for _, pf := range helpers {
			traced := allHelperTraced[pf.RelPath]
			if len(traced) > 0 {
				cout.Verbosef("    <darkGray>%s</><white;op=bold>%s</> →\n", pf.Dir, pf.Name)
				for _, tpf := range traced {
					cout.Verbosef("      %s\n", tpf.ColouredFileName())
				}
			} else {
				cout.Verbosef("    <darkGray>%s</><white;op=bold>%s</> → <darkGray>no resource files traced</>\n", pf.Dir, pf.Name)
			}
		}
	}

	if len(crossPkgHelpers) > 0 {
		pkgSymbols := map[string]map[string]bool{}
		for _, pf := range crossPkgHelpers {
			dir := filepath.ToSlash(filepath.Dir(pf.RelPath))
			pkgPath := dc.ModulePath + "/" + dir

			symbols := pf.Symbols(true)
			if len(symbols) == 0 {
				cout.Verbosef("    <darkGray>%s</><white;op=bold>%s</> → <darkGray>no exported symbols</>\n", pf.Dir, pf.Name)
				continue
			}
			if pkgSymbols[pkgPath] == nil {
				pkgSymbols[pkgPath] = map[string]bool{}
			}
			for _, s := range symbols {
				pkgSymbols[pkgPath][s] = true
			}
			clog.Log.Debugf("    %s exports: %v", pf.RelPath, symbols)
		}

		tracedDirs := dc.traceImportsToResourceFiles(crossPkgHelpers, pkgSymbols)

		for dir, files := range tracedDirs {
			var prefixes []string
			for _, f := range files {
				tpf := provider.NewFileWithPath(f, dc.RepoPath)
				prefixes = append(prefixes, tpf.ResourcePrefix())
			}
			discovered, err := dc.findLocalTestFiles(dir, prefixes)
			if err != nil {
				clog.Log.Debugf("  failed to find test files in %s: %v", dir, err)
				continue
			}
			for _, pf := range discovered {
				dc.AddTestFile(pf, "TRACED")
			}
		}

		crossPkgTracedFiles := map[string]bool{}
		for _, prefixes := range tracedDirs {
			for _, p := range prefixes {
				crossPkgTracedFiles[p] = true
			}
		}
		if cout.Level >= cout.VerbosityVerbose {
			cout.Printf("  tracing symbols from <yellow>%d</> cross-package helper file(s)...\n", len(crossPkgHelpers))
		} else {
			cout.Printf("  tracing symbols from <yellow>%d</> cross-package helper file(s)... <cyan>%d</> resource file(s)\n", len(crossPkgHelpers), len(crossPkgTracedFiles))
		}

		for _, pf := range crossPkgHelpers {
			var tracedFiles []string
			for _, files := range tracedDirs {
				tracedFiles = append(tracedFiles, files...)
			}
			if len(tracedFiles) > 0 {
				cout.Verbosef("    <darkGray>%s</><white;op=bold>%s</> →\n", pf.Dir, pf.Name)
				for _, t := range tracedFiles {
					tpf := provider.NewFileWithPath(t, dc.RepoPath)
					cout.Verbosef("      %s\n", tpf.ColouredFileName())
				}
			} else {
				cout.Verbosef("    <darkGray>%s</><white;op=bold>%s</> → <darkGray>no resource files traced</>\n", pf.Dir, pf.Name)
			}
		}
	}
}

func (dc *AstDiscoveryContext) TraceVendorFiles(vendorFiles []provider.File) {
	if len(vendorFiles) == 0 || dc.Config.LocalTraceDepth == 0 || dc.Config.LocalVendorMode != "basic" {
		return
	}

	vendorPkgs := map[string]bool{}
	vendorFileToPkg := map[string]string{}
	pkgToResources := map[string][]provider.File{}

	for _, pf := range vendorFiles {
		f := pf.RelPath
		pkgImportPath := filepath.ToSlash(filepath.Dir(strings.TrimPrefix(f, "vendor/")))
		vendorPkgs[pkgImportPath] = true
		vendorFileToPkg[f] = pkgImportPath
		clog.Log.Debugf("    vendor package: %s", pkgImportPath)
	}

	for _, pf := range vendorFiles {
		cout.Verbosef("    <darkGray>%s</><fg=177>%s</> → package <fg=177>%s</>\n",
			pf.Dir, pf.Name, vendorFileToPkg[pf.RelPath])
	}

	servicesDir := filepath.Join(dc.RepoPath, "internal", "services")
	_ = filepath.WalkDir(servicesDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}

		relPath, relErr := filepath.Rel(dc.RepoPath, path)
		if relErr != nil {
			return nil
		}
		relPath = filepath.ToSlash(relPath)

		if !dc.Config.FileRegEx.MatchString(relPath) {
			return nil
		}

		fset := token.NewFileSet()
		parsed, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return nil
		}

		for _, imp := range parsed.Imports {
			impPath := strings.Trim(imp.Path.Value, `"`)
			if !vendorPkgs[impPath] {
				continue
			}

			dir := filepath.ToSlash(filepath.Dir(relPath))
			tracedFile := provider.NewFileWithPath(relPath, dc.RepoPath)
			clog.Log.Debugf("    vendor traced: %s imports %s", relPath, impPath)

			pkgToResources[impPath] = append(pkgToResources[impPath], tracedFile)

			discovered, findErr := dc.findLocalTestFiles(dir, []string{tracedFile.ResourcePrefix()})
			if findErr != nil {
				return nil
			}
			for _, pf := range discovered {
				dc.AddTestFile(pf, "VENDOR")
			}
			break
		}

		return nil
	})

	vendorTracedCount := 0
	for _, resources := range pkgToResources {
		vendorTracedCount += len(resources)
	}
	if cout.Level >= cout.VerbosityVerbose {
		cout.Printf("  tracing imports from <yellow>%d</> vendor file(s)...\n", len(vendorFiles))
	} else {
		cout.Printf("  tracing imports from <yellow>%d</> vendor file(s)... <cyan>%d</> resource file(s)\n", len(vendorFiles), vendorTracedCount)
	}

	for pkg, resources := range pkgToResources {
		cout.Verbosef("    <fg=177>%s</> →\n", pkg)
		for _, rpf := range resources {
			cout.Verbosef("      %s\n", rpf.ColouredFileName())
		}
	}
	if len(pkgToResources) == 0 {
		cout.Verbosef("    <darkGray>no resource files import changed vendor packages</>\n")
	}
}

func (dc *AstDiscoveryContext) PrintDiscoveredFiles() {
	cout.Printf("  test files: <yellow>%d</>\n", len(dc.TestFiles))
	showTestFiles := dc.Config.CollapseFilesAfter == 0 || len(dc.TestFiles) <= dc.Config.CollapseFilesAfter
	for _, pf := range dc.SortedTestFiles() {
		sources := strings.Join(pf.DiscoveredBy, "+")

		fileColour := provider.FileColourDerived
		for _, s := range pf.DiscoveredBy {
			if s == "CHANGED" {
				fileColour = provider.FileColourTest
				break
			} else if s == "VENDOR" && fileColour != provider.FileColourTest {
				fileColour = provider.FileColourVendor
			} else if s == "TRACED" && fileColour != provider.FileColourTest && fileColour != provider.FileColourVendor {
				fileColour = provider.FileColourHelper
			}
		}

		if showTestFiles {
			cout.Printf("    <darkGray>%s</>%s%s</> <darkGray>[%s]</>\n", pf.Dir, fileColour, pf.Name, sources)
		} else {
			cout.Verbosef("    <darkGray>%s</>%s%s</> <darkGray>[%s]</>\n", pf.Dir, fileColour, pf.Name, sources)
		}
	}
	if !showTestFiles && cout.Level < cout.VerbosityVerbose {
		cout.Printf("    <yellow>%d</> <fg=208>exceeds display limit of</> <yellow>%d</><darkGray>, use -v or --collapse-files-after 0 to see all</>\n", len(dc.TestFiles), dc.Config.CollapseFilesAfter)
	}
}

func (dc *AstDiscoveryContext) ParseTestsConcurrently() (map[string][]string, error) {
	clog.Log.Debugf("  parsing %d test files locally (max %d concurrent):", len(dc.TestFiles), dc.Config.Concurrency)
	serviceTestMap := map[string]map[string]bool{}
	mu := sync.Mutex{}
	wg := sync.WaitGroup{}
	var errs []error
	sem := make(chan struct{}, dc.Config.Concurrency)

	for _, pf := range dc.SortedTestFiles() {
		wg.Add(1)
		go func(pfile *provider.File) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			tests, err := pfile.ExtractTests(dc.Config.SplitTestsOn, dc.Config.ReappendSplitCharacter)
			if err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
				return
			}

			mu.Lock()
			for _, t := range tests {
				clog.Log.Debugf("test: %s", t)
				if _, ok := serviceTestMap[pfile.Service]; !ok {
					serviceTestMap[pfile.Service] = make(map[string]bool)
				}
				serviceTestMap[pfile.Service][t] = true
			}
			mu.Unlock()
		}(pf)
	}

	wg.Wait()
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	serviceTests := map[string][]string{}
	for service, tests := range serviceTestMap {
		for test := range tests {
			serviceTests[service] = append(serviceTests[service], test)
		}
	}
	return serviceTests, nil
}
