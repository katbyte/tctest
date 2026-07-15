package cli

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/google/go-github/v45/github"
	"github.com/katbyte/tctest/lib/clog"
	"github.com/katbyte/tctest/lib/cout"
	"github.com/katbyte/tctest/lib/git"
)

// PrTestsLocal performs test discovery using a local git clone of the repository.
// When cfg.LocalMode is AST, this is called instead of PrTests (the HTTP-based path).
//
// It fetches the PR merge ref, checks out the code, and uses Go AST to discover
// affected tests — including tracing imports from helper/validation files back to
// resource files to find their tests.
func (gr GithubRepo) PrTestsLocal(pri int, cfg DiscoveryConfig) (*map[string][]string, error) {
	repoPath, err := filepath.Abs(cfg.LocalRepoPath)
	if err != nil {
		return nil, fmt.Errorf("resolving repo path: %w", err)
	}

	// ensure repo path exists, cloning if the directory is empty or doesn't exist
	needsClone := false
	if info, err := os.Stat(repoPath); os.IsNotExist(err) {
		// directory doesn't exist — create it and clone
		if err := os.MkdirAll(repoPath, 0o755); err != nil { //nolint:gosec // directory for user-provided --local-repo-path
			return nil, fmt.Errorf("creating repo path %s: %w", repoPath, err)
		}
		needsClone = true
	} else if err != nil {
		return nil, fmt.Errorf("checking repo path %s: %w", repoPath, err)
	} else if info.IsDir() {
		entries, err := os.ReadDir(repoPath)
		if err != nil {
			return nil, fmt.Errorf("reading repo path %s: %w", repoPath, err)
		}
		if len(entries) == 0 {
			needsClone = true
		}
	}

	if needsClone {
		cloneURL := fmt.Sprintf("https://github.com/%s/%s.git", gr.Owner, gr.Name)
		cout.Printf("  cloning <cyan>%s/%s</> into <darkGray>%s</>...\n", gr.Owner, gr.Name, repoPath)
		if err := git.Clone(filepath.Dir(repoPath), cloneURL, repoPath); err != nil {
			return nil, fmt.Errorf("cloning repo: %w", err)
		}
	}

	// verify repo path is a git repo
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err != nil {
		return nil, fmt.Errorf("repo path %s is not a git repository: %w", repoPath, err)
	}

	// abort if there are uncommitted changes
	cout.Printf("  local AST detection: <darkGray>%s</> trace depth <yellow>%d</>\n", repoPath, cfg.LocalTraceDepth)
	if err := git.EnsureCleanWorkingTree(repoPath); err != nil {
		return nil, err
	}

	// fetch PR merge ref + checkout
	cout.Printf("  fetching PR <cyan>#%d</> merge ref...\n", pri)
	if err := git.FetchPRMergeRef(repoPath, pri); err != nil {
		return nil, fmt.Errorf("failed to fetch PR merge ref: %w", err)
	}
	if err := git.CheckoutFetchHead(repoPath); err != nil {
		return nil, fmt.Errorf("failed to checkout merge commit: %w", err)
	}

	// check PR state via GitHub API
	client, ctx := gr.NewClient()
	clog.Log.Debugf("fetching data for PR %s/%s/#%d...", gr.Owner, gr.Name, pri)
	pr, _, err := client.PullRequests.Get(ctx, gr.Owner, gr.Name, pri)
	if err != nil {
		return nil, err
	}
	if pr.GetState() == "closed" {
		return nil, errors.New("cannot start build for a closed pr")
	}

	// get module path from go.mod for import tracing
	modulePath, err := getModulePath(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read module path: %w", err)
	}
	clog.Log.Debugf("  module path: %s", modulePath)

	// compile regexes
	filterRegEx := regexp.MustCompile(cfg.FileRegExStr)
	testFileSuffixREs := make([]*regexp.Regexp, 0, len(cfg.AccTestFileSuffixRegexes))
	for _, p := range cfg.AccTestFileSuffixRegexes {
		testFileSuffixREs = append(testFileSuffixREs, regexp.MustCompile(p))
	}

	// categorise changed files from GitHub API
	testFilesToParse := map[string]struct{}{}
	changedTestFiles := map[string]bool{}
	derivedTestFiles := map[string]bool{}
	tracedTestFiles := map[string]bool{}
	vendorTestFiles := map[string]bool{}
	testFileSeen := map[string]bool{}
	var testFilesList []string
	resourceDirs := map[string][]string{} // dir -> resource prefixes
	var helperFiles []string
	helperFileSet := map[string]bool{}
	var vendorFiles []string
	unitTestFiles := map[string]bool{}
	changedFileCount := 0

	// print config
	cout.Verbosef("  file regex: <darkGray>%s</>\n", cfg.FileRegExStr)
	cout.Verbosef("  acctest file suffix patterns: <darkGray>%s</>\n", strings.Join(cfg.AccTestFileSuffixRegexes, ", "))

	// buffer changed file output lines — we don't know the total count until after the callback
	var changedFileLines []string

	err = gr.ListAllPullRequestFiles(pri, func(files []*github.CommitFile, _ *github.Response) error {
		for _, f := range files {
			if f.Filename == nil {
				continue
			}
			name := *f.Filename
			clog.Log.Debugf("    %v (%s)", name, f.GetStatus())

			if !strings.HasSuffix(name, ".go") {
				continue
			}
			if f.GetStatus() == "removed" {
				clog.Log.Debugf("    skipping removed file: %s", name)
				continue
			}

			dir := name[:strings.LastIndex(name, "/")+1]
			base := name[strings.LastIndex(name, "/")+1:]

			// skip registration/resourceids in service directories
			if (strings.Contains(name, "/services/") || strings.Contains(name, "/service/")) &&
				(strings.HasSuffix(name, "registration.go") || strings.HasSuffix(name, "resourceids.go")) {
				continue
			}

			// test file — check if it contains TestAcc functions
			if strings.HasSuffix(name, "_test.go") {
				changedFileCount++

				// quick local read to check for TestAcc
				hasAccTests := false
				if content, readErr := os.ReadFile(filepath.Join(repoPath, name)); readErr == nil { //nolint:gosec // path is from user-provided --local-repo-path flag
					hasAccTests = strings.Contains(string(content), "func TestAcc")
				}

				if hasAccTests {
					if !testFileSeen[name] {
						testFilesList = append(testFilesList, name)
						testFileSeen[name] = true
					}
					changedTestFiles[name] = true
					testFilesToParse[name] = struct{}{}
					changedFileLines = append(changedFileLines, fmt.Sprintf("    <darkGray>%s</><fg=28>%s</> <darkGray>[TEST]</>\n", dir, base))
				} else {
					unitTestFiles[name] = true
					clog.Log.Debugf("    %s: no TestAcc functions, skipping", name)
					changedFileLines = append(changedFileLines, fmt.Sprintf("    <darkGray>%s</><darkGray>%s</> <darkGray>[UNIT]</>\n", dir, base))
				}
				continue
			}

			// resource file — matches fileregex
			if filterRegEx.MatchString(name) {
				changedFileCount++
				nameNoExt := strings.TrimSuffix(name, ".go")
				rDir := nameNoExt[:strings.LastIndex(nameNoExt, "/")]
				rBase := nameNoExt[strings.LastIndex(nameNoExt, "/")+1:]
				resourceName := strings.TrimSuffix(rBase, "_resource")
				resourceDirs[rDir] = append(resourceDirs[rDir], resourceName)
				changedFileLines = append(changedFileLines, fmt.Sprintf("    <darkGray>%s/</><fg=36>%s.go</> <darkGray>[RESOURCE]</>\n", rDir, rBase))
				continue
			}

			// helper file — in service dir but doesn't match fileregex
			if strings.Contains(name, "/services/") || strings.Contains(name, "/service/") {
				changedFileCount++
				helperFiles = append(helperFiles, name)
				helperFileSet[name] = true
				changedFileLines = append(changedFileLines, fmt.Sprintf("    <darkGray>%s</><fg=117>%s</> <darkGray>[HELPER]</>\n", dir, base))
				continue
			}

			// vendor file
			if strings.HasPrefix(name, "vendor/") {
				changedFileCount++
				vendorFiles = append(vendorFiles, name)
				changedFileLines = append(changedFileLines, fmt.Sprintf("    <darkGray>%s</><fg=177>%s</> <darkGray>[VENDOR]</>\n", dir, base))
				continue
			}

			// file outside service directories
			changedFileCount++
			changedFileLines = append(changedFileLines, fmt.Sprintf("    <darkGray>%s</><darkGray>%s</> <darkGray>[OTHER]</>\n", dir, base))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get PR files: %w", err)
	}

	cout.Printf("  changed files: <yellow>%d</>\n", changedFileCount)
	showChangedFiles := cfg.CollapseFilesAfter == 0 || changedFileCount <= cfg.CollapseFilesAfter
	for _, line := range changedFileLines {
		if showChangedFiles {
			cout.Printf("%s", line)
		} else {
			cout.Verbosef("%s", line)
		}
	}
	if !showChangedFiles && cout.Level < cout.VerbosityVerbose {
		cout.Printf("    <yellow>%d</> <fg=208>exceeds display limit of</> <yellow>%d</><darkGray>, use -v or --collapse-files-after 0 to see all</>\n", changedFileCount, cfg.CollapseFilesAfter)
	}

	// find sibling test files for resource files (local FS walk)
	for dir, prefixes := range resourceDirs {
		localDir := filepath.Join(repoPath, dir)
		discovered, err := findLocalTestFiles(localDir, dir, prefixes, testFileSuffixREs)
		if err != nil {
			clog.Log.Debugf("  failed to find test files in %s: %v", dir, err)
			continue
		}
		for _, tf := range discovered {
			if testFileSeen[tf] {
				continue
			}
			testFilesToParse[tf] = struct{}{}
			derivedTestFiles[tf] = true
			testFilesList = append(testFilesList, tf)
			testFileSeen[tf] = true
		}
	}

	// import tracing for helper files
	if len(helperFiles) > 0 && cfg.LocalTraceDepth > 0 {
		// split helpers into same-package (dir contains resource files) and cross-package (sub-dir)
		var crossPkgHelpers []string
		samePkgHelpers := map[string][]string{} // dir -> helper files in that dir
		for _, f := range helperFiles {
			dir := filepath.ToSlash(filepath.Dir(f))

			// check if this directory contains any resource files (matching fileregex)
			isSamePkg := false
			if entries, err := os.ReadDir(filepath.Join(repoPath, dir)); err == nil {
				for _, entry := range entries {
					if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
						if filterRegEx.MatchString(filepath.ToSlash(filepath.Join(dir, entry.Name()))) {
							isSamePkg = true
							break
						}
					}
				}
			}

			if isSamePkg {
				samePkgHelpers[dir] = append(samePkgHelpers[dir], f)
			} else {
				crossPkgHelpers = append(crossPkgHelpers, f)
			}
		}

		// same-package tracing: check resource files for direct symbol references
		samePkgHelperCount := 0
		for _, h := range samePkgHelpers {
			samePkgHelperCount += len(h)
		}
		samePkgTracedFiles := map[string]bool{}
		allHelperTraced := map[string][]string{} // helper file -> traced resource files (across all dirs)
		for dir, helpers := range samePkgHelpers {
			// extract all symbols (including unexported) from each helper
			symbols := map[string]bool{}
			for _, f := range helpers {
				for _, s := range extractSymbols(filepath.Join(repoPath, f), false) {
					symbols[s] = true
				}
				clog.Log.Debugf("    same-pkg helper %s symbols: %v", f, symbols)
			}
			if len(symbols) == 0 {
				for _, f := range helpers {
					fDir := f[:strings.LastIndex(f, "/")+1]
					fBase := f[strings.LastIndex(f, "/")+1:]
					cout.Verbosef("    <darkGray>%s</><white;op=bold>%s</> → <darkGray>no symbols found</>\n", fDir, fBase)
				}
				continue
			}

			// scan resource files in the same directory for references
			localDir := filepath.Join(repoPath, dir)
			entries, err := os.ReadDir(localDir)
			if err != nil {
				continue
			}

			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
					continue
				}
				relPath := filepath.ToSlash(filepath.Join(dir, entry.Name()))
				if !filterRegEx.MatchString(relPath) {
					continue // not a resource file
				}

				// check if this resource file references any helper symbols
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
					base := strings.TrimSuffix(entry.Name(), ".go")
					resourceName := strings.TrimSuffix(base, "_resource")
					clog.Log.Debugf("    same-pkg traced: %s uses %v", relPath, usedSymbols)

					for _, f := range helpers {
						allHelperTraced[f] = append(allHelperTraced[f], relPath)
					}
					samePkgTracedFiles[relPath] = true

					// find test files for this resource
					discovered, err := findLocalTestFiles(localDir, dir, []string{resourceName}, testFileSuffixREs)
					if err != nil {
						continue
					}
					for _, tf := range discovered {
						tracedTestFiles[tf] = true
						if testFileSeen[tf] {
							continue
						}
						testFilesToParse[tf] = struct{}{}
						testFilesList = append(testFilesList, tf)
						testFileSeen[tf] = true
					}
				}
			}
		}

		// print same-package tracing summary before verbose details
		if samePkgHelperCount > 0 {
			if cout.Level >= cout.VerbosityVerbose {
				cout.Printf("  tracing symbols from <yellow>%d</> same-package helper file(s)...\n", samePkgHelperCount)
			} else {
				cout.Printf("  tracing symbols from <yellow>%d</> same-package helper file(s)... <cyan>%d</> resource file(s)\n", samePkgHelperCount, len(samePkgTracedFiles))
			}
		}

		// print results: helper file →, then indented traced files
		for _, helpers := range samePkgHelpers {
			for _, f := range helpers {
				traced := allHelperTraced[f]
				if len(traced) > 0 {
					fDir := f[:strings.LastIndex(f, "/")+1]
					fBase := f[strings.LastIndex(f, "/")+1:]
					cout.Verbosef("    <darkGray>%s</><white;op=bold>%s</> →\n", fDir, fBase)
					for _, t := range traced {
						tDir := t[:strings.LastIndex(t, "/")+1]
						tBase := t[strings.LastIndex(t, "/")+1:]
						cout.Verbosef("      <darkGray>%s</><fg=36>%s</>\n", tDir, tBase)
					}
				} else {
					fDir := f[:strings.LastIndex(f, "/")+1]
					fBase := f[strings.LastIndex(f, "/")+1:]
					cout.Verbosef("    <darkGray>%s</><white;op=bold>%s</> → <darkGray>no resource files traced</>\n", fDir, fBase)
				}
			}
		}

		// cross-package tracing (sub-directory helpers)
		if len(crossPkgHelpers) > 0 {
			// parse each cross-package helper file to extract exported symbols
			pkgSymbols := map[string]map[string]bool{}
			for _, f := range crossPkgHelpers {
				localPath := filepath.Join(repoPath, f)
				dir := filepath.ToSlash(filepath.Dir(f))
				pkgPath := modulePath + "/" + dir

				symbols := extractSymbols(localPath, true)
				if len(symbols) == 0 {
					fDir := f[:strings.LastIndex(f, "/")+1]
					fBase := f[strings.LastIndex(f, "/")+1:]
					cout.Verbosef("    <darkGray>%s</><white;op=bold>%s</> → <darkGray>no exported symbols</>\n", fDir, fBase)
					continue
				}
				if pkgSymbols[pkgPath] == nil {
					pkgSymbols[pkgPath] = map[string]bool{}
				}
				for _, s := range symbols {
					pkgSymbols[pkgPath][s] = true
				}
				clog.Log.Debugf("    %s exports: %v", f, symbols)
			}

			tracedDirs := traceImportsToResourceFiles(repoPath, modulePath, crossPkgHelpers, pkgSymbols, filterRegEx, cfg.LocalTraceDepth)

			for dir, files := range tracedDirs {
				localDir := filepath.Join(repoPath, dir)
				// derive test prefixes from actual filenames
				var prefixes []string
				for _, f := range files {
					base := strings.TrimSuffix(filepath.Base(f), ".go")
					prefix := strings.TrimSuffix(base, "_resource")
					prefixes = append(prefixes, prefix)
				}
				discovered, err := findLocalTestFiles(localDir, dir, prefixes, testFileSuffixREs)
				if err != nil {
					clog.Log.Debugf("  failed to find test files in %s: %v", dir, err)
					continue
				}
				for _, tf := range discovered {
					tracedTestFiles[tf] = true
					if testFileSeen[tf] {
						continue
					}
					testFilesToParse[tf] = struct{}{}
					testFilesList = append(testFilesList, tf)
					testFileSeen[tf] = true
				}
			}

			// count unique traced resource files from cross-package helpers
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

			// print cross-package trace results: helper file →, then indented traced files
			for _, f := range crossPkgHelpers {
				var tracedFiles []string
				for _, files := range tracedDirs {
					tracedFiles = append(tracedFiles, files...)
				}
				if len(tracedFiles) > 0 {
					fDir := f[:strings.LastIndex(f, "/")+1]
					fBase := f[strings.LastIndex(f, "/")+1:]
					cout.Verbosef("    <darkGray>%s</><white;op=bold>%s</> →\n", fDir, fBase)
					for _, t := range tracedFiles {
						tDir := t[:strings.LastIndex(t, "/")+1]
						tBase := t[strings.LastIndex(t, "/")+1:]
						cout.Verbosef("      <darkGray>%s</><fg=36>%s</>\n", tDir, tBase)
					}
				} else {
					fDir := f[:strings.LastIndex(f, "/")+1]
					fBase := f[strings.LastIndex(f, "/")+1:]
					cout.Verbosef("    <darkGray>%s</><white;op=bold>%s</> → <darkGray>no resource files traced</>\n", fDir, fBase)
				}
			}
		}
	}

	// vendor file tracing — find resource files that import changed vendor packages
	// Go requires imports to be used, so import presence = definite usage.
	// This is already per-file precise: only resources importing the specific vendor package are flagged.
	if len(vendorFiles) > 0 && cfg.LocalTraceDepth > 0 && cfg.LocalVendorMode == "basic" {
		// (count printed after tracing completes)

		// collect unique vendor package import paths, tracking which files belong to each package
		vendorPkgs := map[string]bool{}
		vendorFileToPkg := map[string]string{} // vendor file -> package import path
		for _, f := range vendorFiles {
			pkgImportPath := filepath.ToSlash(filepath.Dir(strings.TrimPrefix(f, "vendor/")))
			vendorPkgs[pkgImportPath] = true
			vendorFileToPkg[f] = pkgImportPath
			clog.Log.Debugf("    vendor package: %s", pkgImportPath)
		}

		// print vendor file -> package mapping
		for _, f := range vendorFiles {
			base := f[strings.LastIndex(f, "/")+1:]
			cout.Verbosef("    <darkGray>%s</><fg=177>%s</> → package <fg=177>%s</>\n",
				f[:strings.LastIndex(f, "/")+1], base, vendorFileToPkg[f])
		}

		// walk all service directories looking for resource files that import these vendor packages
		// track which packages traced to which resource files
		pkgToResources := map[string][]string{} // package -> list of resource files
		servicesDir := filepath.Join(repoPath, "internal", "services")
		_ = filepath.WalkDir(servicesDir, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil || d.IsDir() {
				//nolint:nilerr // WalkDir: skip dirs and errors, continue walking
				return nil
			}
			if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
				return nil
			}

			relPath, relErr := filepath.Rel(repoPath, path)
			if relErr != nil {
				//nolint:nilerr // filepath.Rel failure is non-fatal, skip this file
				return nil
			}
			relPath = filepath.ToSlash(relPath)

			if !filterRegEx.MatchString(relPath) {
				return nil // only interested in resource files
			}

			// parse imports only (fast)
			fset := token.NewFileSet()
			parsed, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
			if parseErr != nil {
				//nolint:nilerr // parse failure is non-fatal, skip this file
				return nil
			}

			for _, imp := range parsed.Imports {
				impPath := strings.Trim(imp.Path.Value, `"`)
				if !vendorPkgs[impPath] {
					continue
				}

				dir := filepath.ToSlash(filepath.Dir(relPath))
				nameNoExt := strings.TrimSuffix(filepath.Base(relPath), ".go")
				resourceName := strings.TrimSuffix(nameNoExt, "_resource")
				clog.Log.Debugf("    vendor traced: %s imports %s", relPath, impPath)

				pkgToResources[impPath] = append(pkgToResources[impPath], relPath)

				localDir := filepath.Join(repoPath, dir)
				discovered, findErr := findLocalTestFiles(localDir, dir, []string{resourceName}, testFileSuffixREs)
				if findErr != nil {
					//nolint:nilerr // test file discovery failure is non-fatal
					return nil
				}
				for _, tf := range discovered {
					vendorTestFiles[tf] = true
					if testFileSeen[tf] {
						continue
					}
					testFilesToParse[tf] = struct{}{}
					testFilesList = append(testFilesList, tf)
					testFileSeen[tf] = true
				}
				break // found a matching import, no need to check others
			}

			return nil
		})

		// count unique traced resource files from vendor tracing
		vendorTracedCount := 0
		for _, resources := range pkgToResources {
			vendorTracedCount += len(resources)
		}
		if cout.Level >= cout.VerbosityVerbose {
			cout.Printf("  tracing imports from <yellow>%d</> vendor file(s)...\n", len(vendorFiles))
		} else {
			cout.Printf("  tracing imports from <yellow>%d</> vendor file(s)... <cyan>%d</> resource file(s)\n", len(vendorFiles), vendorTracedCount)
		}

		// print which packages traced to which resource files
		for pkg, resources := range pkgToResources {
			cout.Verbosef("    <fg=177>%s</> →\n", pkg)
			for _, res := range resources {
				resDir := res[:strings.LastIndex(res, "/")+1]
				resBase := res[strings.LastIndex(res, "/")+1:]
				cout.Verbosef("      <darkGray>%s</><fg=36>%s</>\n", resDir, resBase)
			}
		}
		if len(pkgToResources) == 0 {
			cout.Verbosef("    <darkGray>no resource files import changed vendor packages</>\n")
		}
	}

	// print all test files with combined labels
	cout.Printf("  test files: <yellow>%d</>\n", len(testFilesList))
	showTestFiles := cfg.CollapseFilesAfter == 0 || len(testFilesList) <= cfg.CollapseFilesAfter
	for _, tf := range testFilesList {
		tfDir := tf[:strings.LastIndex(tf, "/")+1]
		tfBase := tf[strings.LastIndex(tf, "/")+1:]

		var labels []string
		if changedTestFiles[tf] {
			labels = append(labels, "CHANGED")
		}
		if derivedTestFiles[tf] {
			labels = append(labels, "DERIVED")
		}
		if tracedTestFiles[tf] {
			labels = append(labels, "TRACED")
		}
		if vendorTestFiles[tf] {
			labels = append(labels, "VENDOR")
		}
		label := strings.Join(labels, "+")

		// changed = green, vendor = light purple, traced = light blue, derived = cyan
		fileColor := "<fg=36>"
		switch {
		case changedTestFiles[tf]:
			fileColor = "<fg=28>"
		case vendorTestFiles[tf]:
			fileColor = "<fg=177>"
		case tracedTestFiles[tf]:
			fileColor = "<fg=117>"
		}
		if showTestFiles {
			cout.Printf("    <darkGray>%s</>%s%s</> <darkGray>[%s]</>\n", tfDir, fileColor, tfBase, label)
		} else {
			cout.Verbosef("    <darkGray>%s</>%s%s</> <darkGray>[%s]</>\n", tfDir, fileColor, tfBase, label)
		}
	}
	if !showTestFiles && cout.Level < cout.VerbosityVerbose {
		cout.Printf("    <yellow>%d</> <fg=208>exceeds display limit of</> <yellow>%d</><darkGray>, use -v or --collapse-files-after 0 to see all</>\n", len(testFilesList), cfg.CollapseFilesAfter)
	}

	// parse test files concurrently
	clog.Log.Debugf("  parsing %d test files locally (max %d concurrent):", len(testFilesToParse), cfg.Concurrency)
	serviceTestMap := map[string]map[string]bool{}
	mu := sync.Mutex{}
	wg := sync.WaitGroup{}
	firstErr := error(nil)
	sem := make(chan struct{}, cfg.Concurrency)

	for tf := range testFilesToParse {
		wg.Add(1)
		go func(f string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			service, tests, err := parseLocalTestFile(repoPath, f, cfg)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}

			mu.Lock()
			for _, t := range tests {
				clog.Log.Debugf("test: %s", t)
				if _, ok := serviceTestMap[service]; !ok {
					serviceTestMap[service] = make(map[string]bool)
				}
				serviceTestMap[service][t] = true
			}
			mu.Unlock()
		}(tf)
	}

	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}

	// convert to result format
	serviceTests := map[string][]string{}
	for service := range serviceTestMap {
		serviceTests[service] = []string{}
		for test := range serviceTestMap[service] {
			serviceInfo := ""
			if service != "" {
				serviceInfo = service + ": "
			}
			clog.Log.Debugf("%s%s", serviceInfo, test)
			serviceTests[service] = append(serviceTests[service], test)
		}
	}

	clog.Log.Debugf("  FOUND %d services", len(serviceTests))
	return &serviceTests, nil
}

// --- Module path ---

// getModulePath reads go.mod in the repo and returns the module import path.
func getModulePath(repoPath string) (string, error) {
	data, err := os.ReadFile(filepath.Join(repoPath, "go.mod")) //nolint:gosec // path is from user-provided --local-repo-path flag
	if err != nil {
		return "", fmt.Errorf("reading go.mod: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", errors.New("module directive not found in go.mod")
}

// --- Local test file discovery ---

// findLocalTestFiles walks a local directory and returns test files matching
// the resource prefix + suffix regex patterns. Same matching logic as the
// GitHub Contents API path, but reads from the local filesystem (no 1000-file cap).
func findLocalTestFiles(localDir, relativeDir string, resourcePrefixes []string, suffixREs []*regexp.Regexp) ([]string, error) {
	entries, err := os.ReadDir(localDir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", localDir, err)
	}

	var testFiles []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		nameNoExt := strings.TrimSuffix(entry.Name(), ".go")
		matched := false
		for _, prefix := range resourcePrefixes {
			if !strings.HasPrefix(nameNoExt, prefix) {
				continue
			}
			remainder := nameNoExt[len(prefix):]
			for _, re := range suffixREs {
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
			testFiles = append(testFiles, relativeDir+"/"+entry.Name())
		}
	}

	return testFiles, nil
}

// --- Local test file parsing ---

// parseLocalTestFile reads a test file from the local filesystem and extracts
// TestAcc* function names using Go AST. Falls back to regex if AST parsing fails.
// Returns (service, testNames, error).
func parseLocalTestFile(repoPath, filePath string, cfg DiscoveryConfig) (string, []string, error) {
	localPath := filepath.Join(repoPath, filePath)
	content, err := os.ReadFile(localPath) //nolint:gosec // path is from user-provided --local-repo-path flag
	if err != nil {
		return "", nil, fmt.Errorf("reading %s: %w", filePath, err)
	}

	// extract test function names via AST
	var tests []string
	fset := token.NewFileSet()
	parsed, parseErr := parser.ParseFile(fset, filePath, content, 0)
	if parseErr != nil {
		clog.Log.Debugf("    failed to parse %s, falling back to regex: %v", filePath, parseErr)
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

	// extract service name from path (handles both /services/ and /service/)
	service := ""
	for _, sep := range []string{"/services/", "/service/"} {
		parts := strings.Split(filePath, sep)
		if len(parts) == 2 {
			service = strings.Split(parts[1], "/")[0]
			break
		}
	}

	// process test names: split and optionally reappend split character
	processedTests := make([]string, 0, len(tests))
	for _, t := range tests {
		testName := strings.Split(strings.Split(t, cfg.SplitTestsOn)[0], "(")[0]
		if cfg.ReappendSplitCharacter && cfg.SplitTestsOn != "" {
			testName += cfg.SplitTestsOn
		}
		processedTests = append(processedTests, testName)
	}

	return service, processedTests, nil
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
// Returns map[dir][]resourcePrefix (same format as resourceDirs in GetAllPullRequestFiles).
func traceImportsToResourceFiles(repoPath, modulePath string, helperFiles []string, pkgSymbols map[string]map[string]bool, filterRegEx *regexp.Regexp, maxDepth int) map[string][]string {
	result := map[string][]string{}

	// collect unique packages of helper files
	currentLevel := map[string]string{} // package import path -> service directory
	visited := map[string]bool{}

	for _, f := range helperFiles {
		dir := filepath.ToSlash(filepath.Dir(f))
		pkgPath := modulePath + "/" + dir

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
	for depth := 0; depth < maxDepth && len(currentLevel) > 0; depth++ {
		nextLevel := map[string]string{}

		for pkgPath, serviceDir := range currentLevel {
			localServiceDir := filepath.Join(repoPath, serviceDir)
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
				relPath, relErr := filepath.Rel(repoPath, path)
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
					if filterRegEx.MatchString(relPath) {
						dir := filepath.ToSlash(filepath.Dir(relPath))
						result[dir] = append(result[dir], relPath)
						clog.Log.Debugf("    traced: %s imports %s (depth %d, package-level)", relPath, pkgPath, depth+1)
					} else {
						helperPkg := modulePath + "/" + filepath.ToSlash(filepath.Dir(relPath))
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
				if filterRegEx.MatchString(relPath) {
					// it's a resource file — add to results
					dir := filepath.ToSlash(filepath.Dir(relPath))
					result[dir] = append(result[dir], relPath)
					clog.Log.Debugf("    traced: %s uses %v from %s (depth %d)", relPath, usedSymbols, pkgPath, depth+1)
				} else {
					// it's another helper — queue for next depth
					helperPkg := modulePath + "/" + filepath.ToSlash(filepath.Dir(relPath))
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

// extractSymbols parses a Go file and returns symbol names (functions, types, variables, constants).
// If exportedOnly is true, only exported (uppercase) symbols are returned.
// If exportedOnly is false, all symbols are returned (for same-package tracing).
func extractSymbols(filePath string, exportedOnly bool) []string {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, filePath, nil, 0)
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
