package cli

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/google/go-github/v89/github"
	"github.com/katbyte/tctest/lib/chttp"
	"github.com/katbyte/tctest/lib/clog"
	"github.com/katbyte/tctest/lib/cout"
	"github.com/katbyte/tctest/lib/gh"
	"github.com/katbyte/tctest/lib/git"
	"github.com/katbyte/tctest/lib/provider"
	"github.com/pkg/browser"
)

// GetPrTests discovers the tests that need to be run for a PR. It first checks if the PR title contains
// a test override. If not, it delegates to GithubRepo.PrTestsFromAPI to discover tests based on changed files.
func (f *FlagData) GetPrTests(number int, title string) (map[string][]string, error) {
	ghr := f.NewRepo()

	prURL := ghr.PrURL(number)
	var serviceTests map[string][]string
	var err error

	mode := f.DiscoveryConfig.Mode
	if strings.EqualFold(mode, "AST") {
		repoPath := f.DiscoveryConfig.LocalRepoPath
		if repoPath == "" {
			cwd, err := os.Getwd()
			if err == nil && git.IsRepoForRemote(cwd, ghr.CloneURL()) {
				repoPath = cwd
			}
		}

		if repoPath != "" {
			f.DiscoveryConfig.LocalRepoPath = repoPath
			cout.Printf("Discovering tests for pr <cyan>#%d</> %s <darkGray>%s</> <yellow>[mode=AST]</>\n", number, title, prURL)
			serviceTests, err = ghr.PrTestsFromAst(number, f.DiscoveryConfig)
		} else {
			cout.Printf("Discovering tests for pr <cyan>#%d</> %s <darkGray>%s</> <yellow>[mode=api (fallback)]</>\n", number, title, prURL)
			serviceTests, err = ghr.PrTestsFromAPI(number, f.DiscoveryConfig)
		}
	} else {
		cout.Printf("Discovering tests for pr <cyan>#%d</> %s <darkGray>%s</> <yellow>[mode=api]</>\n", number, title, prURL)
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
	for service := range serviceTests {
		if len(service) > maxLen {
			maxLen = len(service)
		}
	}

	for service, tests := range serviceTests {
		cout.Printf("  <yellow>%-*s</>: %s\n", maxLen, service, strings.Join(tests, ", "))
	}

	return serviceTests, nil
}

// PrTestsFromAPI fetches the list of files changed in a PR and determines which tests should be run.
// It uses GetPullRequestTestFiles to get the files, groups them into packages, and returns a map of package names to a list of test names.
func (ghr GithubRepo) PrTestsFromAPI(pri int, cfg DiscoveryConfig) (map[string][]string, error) {
	client, ctx := ghr.NewClient()
	httpClient := chttp.NewHTTPClient("HTTP")

	clog.Log.Debugf("fetching data for PR %s/%s/#%d...", ghr.Owner, ghr.Name, pri)
	pr, _, err := client.PullRequests.Get(ctx, ghr.Owner, ghr.Name, pri)
	if err != nil {
		return nil, gh.WrapGitHubError(err, fmt.Sprintf("fetching PR %s/%s/#%d", ghr.Owner, ghr.Name, pri))
	}

	clog.Log.Debugf("  checking pr state: %v", pr.GetState())
	if pr.GetState() == gh.PRStateClosed {
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

			if status == http.StatusNotFound {
				clog.Log.Debugf("    skipping %s (not found at merge commit)", f.RelPath)
				return // file was skipped
			}
			if status != http.StatusOK {
				// anything else (403/429/5xx) means we failed to fetch a file that exists;
				// silently skipping it would trigger builds with an incomplete test regex
				mu.Lock()
				errs = append(errs, fmt.Errorf("downloading %s: unexpected status %d", f.RelPath, status))
				mu.Unlock()
				return
			}

			f.SetContent(content)
			tests, err := f.ExtractTests(cfg.SplitTestsOn, cfg.ReappendSplitCharacter)
			if err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
				return
			}
			service := f.Service

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
		// map iteration order is random; sort so the generated test regex is deterministic
		sort.Strings(serviceTests[service])
	}

	return serviceTests, nil
}

// CheckPrCanBuild verifies a PR exists, is open, and has a merge commit. Used by the
// direct-trigger path (--service + --all/test regex), which skips discovery and would
// otherwise happily trigger builds on a stale or missing refs/pull/N/merge ref.
func (f *FlagData) CheckPrCanBuild(number int) error {
	ghr := f.NewRepo()
	client, ctx := ghr.NewClient()

	pr, _, err := client.PullRequests.Get(ctx, ghr.Owner, ghr.Name, number)
	if err != nil {
		return gh.WrapGitHubError(err, fmt.Sprintf("fetching PR %s/%s/#%d", ghr.Owner, ghr.Name, number))
	}
	if pr.GetState() == gh.PRStateClosed {
		return errors.New("cannot start build for a closed pr")
	}
	if pr.MergeCommitSHA == nil {
		return errors.New("merge commit SHA is nil, is there a merge conflict?")
	}
	return nil
}

// GetPullRequestTestFiles fetches all changed files in a PR and determines the related test files.
// It classifies files based on the DiscoveryConfig and lists contents of directories containing changed resources to find related tests.
func (ghr GithubRepo) GetPullRequestTestFiles(pri int, cfg DiscoveryConfig) ([]provider.File, error) {
	// track resource files that need sibling test file discovery
	// key: directory path, value: list of resource prefixes (e.g. "foo")
	resourcePrefixesByPackage := map[string][]string{}

	// track changed files and test files for output
	var changedServiceFiles []provider.File
	skippedFiles := map[string]bool{} // service files that didn't match the regex

	testFiles := map[string]*provider.File{}
	addTestFile := func(pf provider.File, source string) {
		existing, ok := testFiles[pf.RelPath]
		if !ok {
			existing = &pf
			testFiles[pf.RelPath] = existing
		}
		existing.AddDiscovery(source)
	}

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

				// Azure migration files live in a subdirectory/separate package. These files are _usually_ prefixed with the resource name
				// which can be used to determine a test prefix.
				if pf.IsMigrationFile() {
					parentDir := path.Dir(path.Dir(pf.RelPath))
					resourcePrefixesByPackage[parentDir] = append(resourcePrefixesByPackage[parentDir], pf.MigrationResourcePrefix())
				} else {
					skippedFiles[pf.RelPath] = true
				}
				continue
			}

			if pf.Type == provider.FileTypeTest {
				changedServiceFiles = append(changedServiceFiles, pf)
				addTestFile(pf, "CHANGED")
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
				// a directory new in this PR won't exist on the default branch; anything else
				// would silently drop derived sibling test files
				var ghErr *github.ErrorResponse
				if errors.As(err, &ghErr) && ghErr.Response != nil && ghErr.Response.StatusCode == http.StatusNotFound {
					clog.Log.Debugf("  directory %s not found (new in this PR?), skipping sibling test discovery", dir)
					continue
				}
				return nil, fmt.Errorf("failed to list directory %s for related test files: %w", dir, err)
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

				if _, exists := testFiles[pf.RelPath]; exists {
					continue
				}

				clog.Log.Debugf("    discovered related test: %s", pf.RelPath)
				addTestFile(pf, "DERIVED")
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

	// sort test files
	sortedTestFiles := make([]*provider.File, 0, len(testFiles))
	for _, pf := range testFiles {
		sortedTestFiles = append(sortedTestFiles, pf)
	}
	sort.Slice(sortedTestFiles, func(i, j int) bool {
		return sortedTestFiles[i].RelPath < sortedTestFiles[j].RelPath
	})

	// print test files
	cout.Printf("  test files: <yellow>%d</>\n", len(sortedTestFiles))
	showTestFiles := cfg.CollapseFilesAfter == 0 || len(sortedTestFiles) <= cfg.CollapseFilesAfter
	for _, pf := range sortedTestFiles {
		sources := strings.Join(pf.DiscoveredBy, "+")

		fileColour := provider.FileColourDerived
		if slices.Contains(pf.DiscoveredBy, "CHANGED") {
			fileColour = provider.FileColourTest
		}

		if showTestFiles {
			cout.Printf("    <darkGray>%s</>%s%s</> <darkGray>[%s]</>\n", pf.Dir, fileColour, pf.Name, sources)
		} else {
			cout.Verbosef("    <darkGray>%s</>%s%s</> <darkGray>[%s]</>\n", pf.Dir, fileColour, pf.Name, sources)
		}
	}
	if !showTestFiles && cout.Level < cout.VerbosityVerbose {
		cout.Printf("    <yellow>%d</> <fg=208>exceeds display limit of</> <yellow>%d</><darkGray>, use -v or --collapse-files-after 0 to see all</>\n", len(sortedTestFiles), cfg.CollapseFilesAfter)
	}

	clog.Log.Debugf("  FOUND %d", len(testFiles))
	for f := range testFiles {
		clog.Log.Debugf("     %s", f)
	}

	files := make([]provider.File, 0, len(sortedTestFiles))
	for _, pf := range sortedTestFiles {
		files = append(files, *pf)
	}
	return files, nil
}
