package cli

import (
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"
	"sync"

	"github.com/google/go-github/v45/github"
	"github.com/katbyte/tctest/lib/chttp"
	"github.com/katbyte/tctest/lib/clog"
	"github.com/katbyte/tctest/lib/cout"
	"github.com/katbyte/tctest/lib/provider"
	"github.com/pkg/browser"
)

// GetPrTests discovers the tests that need to be run for a PR. It first checks if the PR title contains
// a test override. If not, it delegates to GithubRepo.PrTestsFromAPI to discover tests based on changed files.
func (f FlagData) GetPrTests(number int, title string) (*map[string][]string, error) {
	ghr := f.NewRepo()

	prURL := ghr.PrURL(number)
	var serviceTests *map[string][]string
	var err error

	if f.DiscoveryConfig.LocalRepoPath != "" && strings.EqualFold(f.DiscoveryConfig.LocalMode, "AST") {
		cout.Printf("Discovering tests for pr <cyan>#%d</> %s <darkGray>%s</> <yellow>[AST]</>\n", number, title, prURL)
		serviceTests, err = ghr.PrTestsFromLocal(number, f.DiscoveryConfig)
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

// PrTestsFromAPI fetches the list of files changed in a PR and determines which tests should be run.
// It uses GetPullRequestTestFiles to get the files, groups them into packages, and returns a map of package names to a list of test names.
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
	filesFiltered, err := ghr.GetPullRequestTestFiles(pri, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR files for %s/%s/pull/%d: %w", ghr.Owner, ghr.Name, pri, err)
	}

	// for each file get content and parse out test files & services
	serviceTestMap := map[string]map[string]bool{}

	clog.Log.Debugf("  downloading & parsing %d files concurrently (max %d):", len(filesFiltered), cfg.Concurrency)
	mu := sync.Mutex{}
	wg := sync.WaitGroup{}
	var errs []error
	sem := make(chan struct{}, cfg.Concurrency)

	for _, f := range filesFiltered {
		wg.Add(1)
		go func(f provider.File) {
			defer wg.Done()
			sem <- struct{}{}        // acquire semaphore
			defer func() { <-sem }() // release semaphore

			content, status, err := ghr.DownloadFile(ctx, httpClient, f.RelPath, *pr.MergeCommitSHA)
			if err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
				return
			}

			if status != http.StatusOK {
				clog.Log.Debugf("    skipping %s (not found at merge commit, status %d)", f.RelPath, status)
				return // file was skipped
			}

			f.SetContent(content)
			tests, err := f.ExtractTests(cfg.SplitTestsOn, cfg.ReappendSplitCharacter)
			if err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
				return
			}
			service := f.ExtractService()

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

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
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

// GetPullRequestTestFiles fetches all changed files in a PR and determines the related test files.
// It classifies files based on the DiscoveryConfig and lists contents of directories containing changed resources to find related tests.
func (ghr GithubRepo) GetPullRequestTestFiles(pri int, cfg DiscoveryConfig) ([]provider.File, error) {
	result := make(map[string]provider.File)

	// track resource files that need sibling test file discovery
	// key: directory path, value: list of resource prefixes (e.g. "foo")
	resourcePrefixesByPackage := map[string][]string{}

	// track changed files and test files for output
	var changedServiceFiles []provider.File
	skippedFiles := map[string]bool{} // service files that didn't match the regex
	var testFiles []provider.File
	changedTestFiles := map[string]bool{} // tracks which test files came from the PR diff
	derivedTestFiles := map[string]bool{} // tracks which test files were derived
	testFileSeen := map[string]bool{}     // dedup test files

	// first get all files for the pull request and filter out every one that is not inside a service package
	err := ghr.ListAllPullRequestFiles(pri, func(files []*github.CommitFile, _ *github.Response) error {
		for _, f := range files {
			if f.Filename == nil {
				continue
			}

			pf := provider.NewFile(f.GetFilename())
			clog.Log.Debugf("    %v (%s)", pf.RelPath, f.GetStatus())

			// for now we only care about go files, data files that acctests load/rely on will be skipped for now
			if !strings.HasSuffix(pf.RelPath, ".go") {
				continue
			}

			// skip deleted files - they won't exist at the merge commit
			if f.GetStatus() == "removed" {
				clog.Log.Debugf("    skipping removed file: %s", pf.RelPath)
				continue
			}

			if pf.Type == provider.FileTypeHelper {
				// track service files that don't match the regex (e.g. client helpers)
				changedServiceFiles = append(changedServiceFiles, pf)
				skippedFiles[pf.RelPath] = true
				continue
			}

			if pf.Type == provider.FileTypeTest {
				changedServiceFiles = append(changedServiceFiles, pf)
				if !testFileSeen[pf.RelPath] {
					testFiles = append(testFiles, pf)
					testFileSeen[pf.RelPath] = true
				}
				changedTestFiles[pf.RelPath] = true
				result[pf.RelPath] = pf
				continue
			}

			if pf.Type == provider.FileTypeOther || pf.Type == provider.FileTypeVendor {
				// if they are in the service path (e.g. registration.go, resourceids.go), mark them as skipped in the output
				if pf.InServicePackage() {
					changedServiceFiles = append(changedServiceFiles, pf)
					skippedFiles[pf.RelPath] = true
				}
				continue
			}

			changedServiceFiles = append(changedServiceFiles, pf)

			// note the directory and probable resourceName so we can discover all related test files
			resourcePrefixesByPackage[path.Dir(pf.RelPath)] = append(resourcePrefixesByPackage[path.Dir(pf.RelPath)], pf.ResourcePrefix())
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get all files for %s/%s/pull/%d: %w", ghr.Owner, ghr.Name, pri, err)
	}

	// For each directory containing a modified file, list all files
	// and add test files whose name matches "{resource/datasource-name}{acctest-pattern}.go".
	if len(resourcePrefixesByPackage) > 0 {
		client, ctx := ghr.NewClient()
		for dir, prefixes := range resourcePrefixesByPackage {
			clog.Log.Debugf("  listing directory %s for related test files...", dir)
			_, dirContents, _, err := client.Repositories.GetContents(ctx, ghr.Owner, ghr.Name, dir, nil)
			if err != nil {
				clog.Log.Debugf("  failed to list directory %s: %v", dir, err)
				continue
			}

			for _, entry := range dirContents {
				pf := provider.NewFile(path.Join(dir, entry.GetName()))
				if pf.Type != provider.FileTypeTest {
					continue
				}

				shouldInclude := false
				for _, resource := range prefixes {
					if !strings.HasPrefix(pf.BaseName, resource) {
						continue
					}

					remainder := pf.BaseName[len(resource):]
					for _, testSuffix := range cfg.AccTestFileSuffixRegexes {
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

				if _, exists := result[pf.RelPath]; exists {
					continue
				}

				clog.Log.Debugf("    discovered related test: %s", pf.RelPath)
				result[pf.RelPath] = pf
				derivedTestFiles[pf.RelPath] = true
				if !testFileSeen[pf.RelPath] {
					testFiles = append(testFiles, pf)
					testFileSeen[pf.RelPath] = true
				}
			}
		}
	}

	// print file regex and changed files
	cout.Verbosef("  file regex: <darkGray>%s</>\n", cfg.FileRegEx.String())
	cout.Verbosef("  acctest file suffix patterns: <darkGray>%s</>\n", cfg.AccTestFileSuffixRegexStrings())
	cout.Printf("  changed service package files: <yellow>%d</>\n", len(changedServiceFiles))

	showFiles := cfg.CollapseFilesAfter == 0 || len(changedServiceFiles) <= cfg.CollapseFilesAfter
	for _, pf := range changedServiceFiles {
		// skipped files in red, test files in green, resource files in teal
		colour := pf.TextColour()
		if skippedFiles[pf.RelPath] {
			colour = provider.FileColourSkipped
		}

		if showFiles {
			cout.Printf("    <darkGray>%s</>%s%s</>\n", pf.Dir, colour, pf.Name)
		} else {
			cout.Verbosef("    <darkGray>%s</>%s%s</>\n", pf.Dir, colour, pf.Name)
		}
	}
	if !showFiles && cout.Level < cout.VerbosityVerbose {
		cout.Printf("    <yellow>%d</> <fg=208>exceeds display limit of</> <yellow>%d</><darkGray>, use -v or --collapse-files-after 0 to see all</>\n", len(changedServiceFiles), cfg.CollapseFilesAfter)
	}

	// print test files
	cout.Printf("  test files: <yellow>%d</>\n", len(testFiles))
	showTestFiles := cfg.CollapseFilesAfter == 0 || len(testFiles) <= cfg.CollapseFilesAfter
	for _, pf := range testFiles {
		// build label based on whether file is changed, derived, or both
		var labels []string
		if changedTestFiles[pf.RelPath] {
			labels = append(labels, "CHANGED")
		}
		if derivedTestFiles[pf.RelPath] {
			labels = append(labels, "DERIVED")
		}
		label := strings.Join(labels, "/")

		// changed files in green, derived-only in dark cyan
		fileColour := provider.FileColourDerived
		if changedTestFiles[pf.RelPath] {
			fileColour = provider.FileColourTest
		}
		if showTestFiles {
			cout.Printf("    <darkGray>%s</>%s%s</> <darkGray>[%s]</>\n", pf.Dir, fileColour, pf.Name, label)
		} else {
			cout.Verbosef("    <darkGray>%s</>%s%s</> <darkGray>[%s]</>\n", pf.Dir, fileColour, pf.Name, label)
		}
	}
	if !showTestFiles && cout.Level < cout.VerbosityVerbose {
		cout.Printf("    <yellow>%d</> <fg=208>exceeds display limit of</> <yellow>%d</><darkGray>, use -v or --collapse-files-after 0 to see all</>\n", len(testFiles), cfg.CollapseFilesAfter)
	}

	clog.Log.Debugf("  FOUND %d", len(result))
	for f := range result {
		clog.Log.Debugf("     %s", f)
	}

	files := make([]provider.File, 0, len(result))
	for _, pf := range result {
		files = append(files, pf)
	}
	return files, nil
}
