package cli

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/google/go-github/v45/github"
	"github.com/katbyte/tctest/lib/chttp"
	"github.com/katbyte/tctest/lib/clog"
	"github.com/katbyte/tctest/lib/cout"
	"github.com/katbyte/tctest/lib/provider"
	"github.com/pkg/browser"
)

// TODO reorg this file

func (f FlagData) GetPrTests(number int, title string) (*map[string][]string, error) {
	ghr := f.NewRepo()

	prURL := ghr.PrURL(number)
	var serviceTests *map[string][]string
	var err error

	if f.DiscoveryConfig.LocalRepoPath != "" && strings.EqualFold(f.DiscoveryConfig.LocalMode, "AST") {
		cout.Printf("Discovering tests for pr <cyan>#%d</> %s <darkGray>%s</> <yellow>[AST]</>\n", number, title, prURL)
		serviceTests, err = ghr.PrTestsLocal(number, f.DiscoveryConfig)
	} else {
		cout.Printf("Discovering tests for pr <cyan>#%d</> %s <darkGray>%s</>\n", number, title, prURL)
		serviceTests, err = ghr.PrTestsFromAPI(number, f.DiscoveryConfig)
	}

	if f.OpenInBrowser {
		if err := browser.OpenURL(prURL); err != nil {
			cout.Printf("failed to open build %s in browser", prURL)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("pr list failed: %w", err)
	}

	maxLen := 0
	for service := range *serviceTests {
		if len(service) > maxLen {
			maxLen = len(service)
		}
	}

	for service, tests := range *serviceTests {
		cout.Printf("  <yellow>%-*s</>: %s\n", maxLen, service, strings.Join(tests, ", "))
	}

	return serviceTests, nil
}

func (ghr GithubRepo) PrTestsFromAPI(pri int, cfg DiscoveryConfig) (*map[string][]string, error) {
	client, ctx := ghr.NewClient()
	httpClient := chttp.NewHTTPClient("HTTP")

	clog.Log.Debugf("fetching data for PR %s/%s/#%d...", ghr.Owner, ghr.Name, pri)
	pr, _, err := client.PullRequests.Get(ctx, ghr.Owner, ghr.Name, pri)
	if err != nil {
		return nil, err
	}

	clog.Log.Debugf("  checking pr state: %v", pr.GetState())
	if pr.GetState() == "closed" {
		return nil, errors.New("cannot start build for a closed pr")
	}
	if pr.MergeCommitSHA == nil {
		return nil, errors.New("merge commit SHA is nil, is there a merge conflict?")
	}

	clog.Log.Tracef("listing files...")
	filesFiltered, err := ghr.GetAllPullRequestFiles(pri, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR files for %s/%s/pull/%d: %w", ghr.Owner, ghr.Name, pri, err)
	}

	// for each file get content and parse out test files & services
	serviceTestMap := map[string]map[string]bool{}

	files := make([]string, 0, len(*filesFiltered))
	for f := range *filesFiltered {
		files = append(files, f)
	}

	clog.Log.Debugf("  downloading & parsing %d files concurrently (max %d):", len(files), cfg.Concurrency)
	mu := sync.Mutex{}
	wg := sync.WaitGroup{}
	firstErr := error(nil)
	sem := make(chan struct{}, cfg.Concurrency)

	for _, f := range files {
		wg.Add(1)
		go func(f string) {
			defer wg.Done()
			sem <- struct{}{}        // acquire semaphore
			defer func() { <-sem }() // release semaphore

			service, tests, err := ghr.downloadAndParseTestFile(ctx, httpClient, f, *pr.MergeCommitSHA, cfg)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}

			if tests == nil {
				return // file was skipped (e.g. not found at merge commit)
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
		}(f)
	}

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

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

	return &serviceTests, nil
}

// downloadAndParseTestFile downloads a single file from GitHub using the raw content URL
// and parses it for acceptance test function names. Returns (service, testNames, nil) on
// success, ("", nil, nil) when the file should be skipped (e.g. not found at merge commit),
// or ("", nil, err) on failure.
//
// This uses raw.githubusercontent.com directly instead of the GitHub Contents API
// (client.Repositories.GetContents) to avoid two issues with that approach:
//  1. GetContents has a 1MB file size limit
//  2. GetContents performs a directory listing for each file request (capped at 1000 entries)
func (ghr GithubRepo) downloadAndParseTestFile(ctx context.Context, httpClient *http.Client, filePath, mergeCommitSHA string, cfg DiscoveryConfig) (string, []string, error) {
	clog.Log.Debugf("    download %s", filePath)

	rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", ghr.Owner, ghr.Name, mergeCommitSHA, filePath)

	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return "", nil, fmt.Errorf("creating request for %s: %w", filePath, err)
	}

	if ghr.Token.Token != nil {
		req.Header.Set("Authorization", "token "+*ghr.Token.Token)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("downloading file (%s): %w", filePath, err)
	}

	content, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return "", nil, fmt.Errorf("reading file (%s): %w", filePath, err)
	}

	if resp.StatusCode != http.StatusOK {
		clog.Log.Debugf("    skipping %s (not found at merge commit, status %d)", filePath, resp.StatusCode)
		return "", nil, nil
	}

	// use go/ast to extract test function names
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

	// extract service name from path
	service := ""
	parts := strings.Split(filePath, "/services/")
	if len(parts) == 2 {
		service = strings.Split(parts[1], "/")[0]
	}

	// process test names: split and optionally reappend split character
	processedTests := make([]string, 0, len(tests))
	for _, t := range tests {
		// split on `(` to make sure we just get the full function name
		testName := strings.Split(strings.Split(t, cfg.SplitTestsOn)[0], "(")[0]

		if cfg.ReappendSplitCharacter && cfg.SplitTestsOn != "" {
			testName += cfg.SplitTestsOn
		}

		processedTests = append(processedTests, testName)
	}

	return service, processedTests, nil
}

func (ghr GithubRepo) ListAllPullRequestFiles(pri int, cb func([]*github.CommitFile, *github.Response) error) error {
	client, ctx := ghr.NewClient()

	opts := &github.ListOptions{
		Page:    1,
		PerPage: 100,
	}

	for {
		clog.Log.Debugf("Listing all files for %s/%s/pull/%d (Page %d)...", ghr.Owner, ghr.Name, pri, opts.Page)
		files, resp, err := client.PullRequests.ListFiles(ctx, ghr.Owner, ghr.Name, pri, opts)
		if err != nil {
			return fmt.Errorf("unable to list files for %s/%s/pull/%d (Page %d): %w", ghr.Owner, ghr.Name, pri, opts.Page, err)
		}

		if err = cb(files, resp); err != nil {
			return fmt.Errorf("callback failed for %s/%s/pull/%d (Page %d): %w", ghr.Owner, ghr.Name, pri, opts.Page, err)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return nil
}

func (ghr GithubRepo) GetAllPullRequestFiles(pri int, cfg DiscoveryConfig) (*map[string]struct{}, error) {
	result := make(map[string]struct{})

	// track resource files that need sibling test file discovery
	// key: directory path, value: list of resource prefixes (e.g. "foo")
	resourceDirs := map[string][]string{}

	// track changed files and test files for output
	var changedFiles []string
	skippedFiles := map[string]bool{} // service files that didn't match the regex
	var testFiles []string
	changedTestFiles := map[string]bool{} // tracks which test files came from the PR diff
	derivedTestFiles := map[string]bool{} // tracks which test files were derived
	testFileSeen := map[string]bool{}     // dedup test files

	err := ghr.ListAllPullRequestFiles(pri, func(files []*github.CommitFile, _ *github.Response) error {
		for _, f := range files {
			if f.Filename == nil {
				continue
			}

			name := *f.Filename
			clog.Log.Debugf("    %v (%s)", name, f.GetStatus())
			// for now we only care about go files, data files that acctests load/rely on will be skipped for now
			if !strings.HasSuffix(name, ".go") {
				continue
			}

			// skip deleted files - they won't exist at the merge commit
			if f.GetStatus() == "removed" {
				clog.Log.Debugf("    skipping removed file: %s", name)
				continue
			}

			// if in service package mode skip some files
			if strings.Contains(name, "/services/") {
				// skip files that don't have meaningful test counterparts
				if strings.HasSuffix(name, "registration.go") || strings.HasSuffix(name, "resourceids.go") {
					continue
				}
			}

			if strings.HasSuffix(name, "_test.go") {
				changedFiles = append(changedFiles, name)
				if !testFileSeen[name] {
					testFiles = append(testFiles, name)
					testFileSeen[name] = true
				}
				changedTestFiles[name] = true
				result[name] = struct{}{}
				continue
			}

			if !cfg.FileRegEx.MatchString(name) {
				// track service files that don't match the regex
				if strings.Contains(name, "/services/") {
					changedFiles = append(changedFiles, name)
					skippedFiles[name] = true
				}
				continue
			}

			changedFiles = append(changedFiles, name)

			// note the directory and probable resourceName so we can discover all related test files
			pf := provider.NewFile(name, provider.FileTypeResource)
			resourceDirs[pf.Dir[:len(pf.Dir)-1]] = append(resourceDirs[pf.Dir[:len(pf.Dir)-1]], pf.ResourcePrefix())
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get all files for %s/%s/pull/%d: %w", ghr.Owner, ghr.Name, pri, err)
	}

	// For each directory containing a modified file, list all files
	// and add test files whose name matches "{resource/datasource-name}{acctest-pattern}.go".
	if len(resourceDirs) > 0 {
		client, ctx := ghr.NewClient()
		for dir, prefixes := range resourceDirs {
			clog.Log.Debugf("  listing directory %s for related test files...", dir)
			_, dirContents, _, err := client.Repositories.GetContents(ctx, ghr.Owner, ghr.Name, dir, nil)
			if err != nil {
				clog.Log.Debugf("  failed to list directory %s: %v", dir, err)
				continue
			}

			for _, entry := range dirContents {
				entryName := entry.GetName()
				if !strings.HasSuffix(entryName, "_test.go") {
					continue
				}
				fileNameWithNoExt := strings.TrimSuffix(entryName, ".go")
				shouldInclude := false
				for _, resource := range prefixes {
					if !strings.HasPrefix(fileNameWithNoExt, resource) {
						continue
					}
					remainder := fileNameWithNoExt[len(resource):]
					for _, testSuffix := range cfg.AccTestFileSuffixREs {
						if testSuffix.MatchString(remainder) {
							shouldInclude = true
							break
						}
					}
					if shouldInclude {
						break
					}
				}
				if !shouldInclude {
					continue
				}
				fullPath := dir + "/" + entryName
				if _, exists := result[fullPath]; exists {
					continue
				}
				clog.Log.Debugf("    discovered related test: %s", fullPath)
				result[fullPath] = struct{}{}
				derivedTestFiles[fullPath] = true
				if !testFileSeen[fullPath] {
					testFiles = append(testFiles, fullPath)
					testFileSeen[fullPath] = true
				}
			}
		}
	}

	// print file regex and changed files
	cout.Verbosef("  file regex: <darkGray>%s</>\n", cfg.FileRegExStr)
	cout.Verbosef("  acctest file suffix patterns: <darkGray>%s</>\n", strings.Join(cfg.AccTestFileSuffixRegexes, ", "))
	showFiles := cfg.CollapseFilesAfter == 0 || len(changedFiles) <= cfg.CollapseFilesAfter
	cout.Printf("  changed files: <yellow>%d</>\n", len(changedFiles))
	for _, f := range changedFiles {
		var fileType provider.FileType
		switch {
		case skippedFiles[f]:
			fileType = provider.FileTypeHelper
		case strings.HasSuffix(f, "_test.go"):
			fileType = provider.FileTypeTest
		default:
			fileType = provider.FileTypeResource
		}
		pf := provider.NewFile(f, fileType)

		// skipped files in red, test files in green, resource files in teal
		colour := pf.Colour()
		if skippedFiles[f] {
			colour = "<red>"
		}
		if showFiles {
			cout.Printf("    <darkGray>%s</>%s%s</>\n", pf.Dir, colour, pf.Base)
		} else {
			cout.Verbosef("    <darkGray>%s</>%s%s</>\n", pf.Dir, colour, pf.Base)
		}
	}
	if !showFiles && cout.Level < cout.VerbosityVerbose {
		cout.Printf("    <yellow>%d</> <fg=208>exceeds display limit of</> <yellow>%d</><darkGray>, use -v or --collapse-files-after 0 to see all</>\n", len(changedFiles), cfg.CollapseFilesAfter)
	}

	// print test files
	cout.Printf("  test files: <yellow>%d</>\n", len(testFiles))
	showTestFiles := cfg.CollapseFilesAfter == 0 || len(testFiles) <= cfg.CollapseFilesAfter
	for _, f := range testFiles {
		pfile := provider.NewFile(f, provider.FileTypeTest)

		// build label based on whether file is changed, derived, or both
		var labels []string
		if changedTestFiles[f] {
			labels = append(labels, "CHANGED")
		}
		if derivedTestFiles[f] {
			labels = append(labels, "DERIVED")
		}
		label := strings.Join(labels, "/")

		// changed files in green, derived-only in dark cyan
		fileColor := "<fg=36>" // dark cyan for derived
		if changedTestFiles[f] {
			fileColor = "<fg=28>" // dark green for changed
		}
		if showTestFiles {
			cout.Printf("    <darkGray>%s</>%s%s</> <darkGray>[%s]</>\n", pfile.Dir, fileColor, pfile.Base, label)
		} else {
			cout.Verbosef("    <darkGray>%s</>%s%s</> <darkGray>[%s]</>\n", pfile.Dir, fileColor, pfile.Base, label)
		}
	}
	if !showTestFiles && cout.Level < cout.VerbosityVerbose {
		cout.Printf("    <yellow>%d</> <fg=208>exceeds display limit of</> <yellow>%d</><darkGray>, use -v or --collapse-files-after 0 to see all</>\n", len(testFiles), cfg.CollapseFilesAfter)
	}

	clog.Log.Debugf("  FOUND %d", len(result))
	for f := range result {
		clog.Log.Debugf("     %s", f)
	}

	return &result, nil
}
