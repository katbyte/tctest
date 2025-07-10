package cli

import (
	"bufio"
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/go-github/v45/github"
	c "github.com/gookit/color" //nolint:misspell
	"github.com/katbyte/tctest/lib/clog"
	"github.com/katbyte/tctest/lib/gh"
	"github.com/pkg/browser"
)

// TODO reorg this file

func (f FlagData) GetPrTests(pr int) (*map[string][]string, error) {
	gr := f.NewRepo()

	prURL := gr.PrURL(pr)
	c.Printf("Discovering tests for pr <cyan>#%d</> <darkGray>(%s)...</>\n", pr, prURL)
	serviceTests, err := gr.PrTests(pr, f.GH.FileRegEx, f.GH.SplitTestsOn)

	if f.OpenInBrowser {
		if err := browser.OpenURL(prURL); err != nil {
			c.Printf("failed to open build %s in browser", prURL)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("pr list failed: %w", err)
	}

	for service, tests := range *serviceTests {
		c.Printf("  <yellow>%s</>:\n", service)
		for _, t := range tests {
			c.Printf("    %s\n", t)
		}
	}

	return serviceTests, nil
}

type PrTestsOptions struct {
	FilterRegExStr string
	SplitTestsAt   string
}

func (gr GithubRepo) PrTestsWithDependencies(ctx context.Context, pri int, opts PrTestsOptions, githubClient gh.GitHubClientInterface, httpClient gh.HTTPClientInterface) (*map[string][]string, error) {
	clog.Log.Debugf("fetching data for PR %s/%s/#%d...", gr.Owner, gr.Name, pri)
	pr, err := gr.getPullRequest(ctx, pri, githubClient)
	if err != nil {
		return nil, err
	}
	if err := gr.validatePRState(pr); err != nil {
		return nil, err
	}
	filesFiltered, err := gr.getAllPullRequestFilesWithClient(ctx, pri, opts.FilterRegExStr, githubClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR files for %s/%s/pull/%d: %w", gr.Owner, gr.Name, pri, err)
	}

	// Parse test files and extract services
	serviceTests, err := gr.parseTestsFromFiles(ctx, *filesFiltered, pr, opts.SplitTestsAt, githubClient, httpClient)
	if err != nil {
		return nil, err
	}

	return serviceTests, nil
}

func (gr GithubRepo) PrTests(pri int, filterRegExStr, splitTestsAt string) (*map[string][]string, error) {
	githubClient, httpClient, ctx := gr.NewClientWithInterfaces()
	opts := PrTestsOptions{
		FilterRegExStr: filterRegExStr,
		SplitTestsAt:   splitTestsAt,
	}
	return gr.PrTestsWithDependencies(ctx, pri, opts, githubClient, httpClient)
}

func (gr GithubRepo) getPullRequest(ctx context.Context, pri int, githubClient gh.GitHubClientInterface) (*github.PullRequest, error) {
	pr, _, err := githubClient.GetPullRequest(ctx, gr.Owner, gr.Name, pri)
	if err != nil {
		return nil, err
	}
	return pr, nil
}

func (gr GithubRepo) validatePRState(pr *github.PullRequest) error {
	clog.Log.Debugf("  checking pr state: %v", *pr.State)
	if pr.State != nil && *pr.State == "closed" {
		return fmt.Errorf("cannot start build for a closed pr")
	}
	return nil
}

func getTestFilesInSamePackage(filename, commitSHA string, githubClient gh.GitHubClientInterface, ctx context.Context, owner string, repo string) ([]string, error) {
	result := []string{}
	directory := filepath.Dir(filename)
	_, contents, _, err := githubClient.GetContents(ctx, owner, repo, directory, &github.RepositoryContentGetOptions{Ref: commitSHA})
	if err != nil {
		return nil, err
	}
	if contents != nil {
		for _, file := range contents {
			if file.Type != nil && *file.Type == "file" && file.Name != nil && strings.HasSuffix(*file.Name, "_test.go") {
				result = append(result, filepath.Join(directory, *file.Name))
			}
		}
	}

	return result, nil
}

func (gr GithubRepo) getAllPullRequestFilesWithClient(ctx context.Context, pri int, filterRegExStr string, githubClient gh.GitHubClientInterface) (*map[string]struct{}, error) {
	result := make(map[string]struct{})
	filterRegEx := regexp.MustCompile(filterRegExStr)

	err := gr.listAllPullRequestFilesWithClient(ctx, pri, githubClient, func(files []*github.CommitFile, _ *github.Response) error {
		filesInMergeCommit := make(map[string]struct{}, 0)
		pr, _, err := githubClient.GetPullRequest(ctx, gr.Owner, gr.Name, pri)
		if err != nil {
			return fmt.Errorf("failed to get PR: %w", err)
		}
		if pr.MergeCommitSHA == nil {
			return fmt.Errorf("merge commit SHA is nil")
		}
		commit, _, err := githubClient.GetCommit(ctx, gr.Owner, gr.Name, *pr.MergeCommitSHA)
		if err != nil {
			return fmt.Errorf("failed to get commit %s for %s/%s: %w", *pr.MergeCommitSHA, gr.Owner, gr.Name, err)
		}
		for _, f := range commit.Files {
			if f.Filename != nil {
				filesInMergeCommit[*f.Filename] = struct{}{}
			}
		}
		for _, f := range files {
			if f.Filename == nil {
				continue
			}

			name := *f.Filename
			clog.Log.Debugf("    %v", *f.Filename)

			// if in service package mode skip some files
			if strings.Contains(name, "/services/") {
				if strings.Contains(name, "/client/") || strings.Contains(name, "/parse/") || strings.Contains(name, "/validate/") {
					continue
				}

				if strings.HasSuffix(name, "registration.go") || strings.HasSuffix(name, "resourceids.go") {
					continue
				}
			}

			if strings.HasSuffix(name, "_test.go") {
				result[name] = struct{}{}
				continue
			}

			if !filterRegEx.MatchString(name) {
				continue
			}

			inferredTestFile := strings.Replace(name, ".go", "_test.go", 1)
			if _, ok := filesInMergeCommit[inferredTestFile]; ok {
				result[inferredTestFile] = struct{}{}
				continue
			}
			c.Printf("No standard-named test file found for '%s' -- Falling back to running all tests in package\n", *f.Filename)
			testFiles, err := getTestFilesInSamePackage(*f.Filename, *pr.MergeCommitSHA, githubClient, ctx, gr.Owner, gr.Name)
			if err != nil {
				clog.Log.Debugf("Failed to get test files in same package for %s: %v", *f.Filename, err)
			} else {
				for _, testFile := range testFiles {
					result[testFile] = struct{}{}
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get all files for %s/%s/pull/%d: %w", gr.Owner, gr.Name, pri, err)
	}

	clog.Log.Debugf("  FOUND %d", len(result))
	for f := range result {
		clog.Log.Debugf("     %s", f)
	}

	return &result, nil
}

// listAllPullRequestFilesWithClient lists all PR files using the injected client
func (gr GithubRepo) listAllPullRequestFilesWithClient(ctx context.Context, pri int, githubClient gh.GitHubClientInterface, cb func([]*github.CommitFile, *github.Response) error) error {
	opts := &github.ListOptions{
		Page:    1,
		PerPage: 100,
	}

	for {
		clog.Log.Debugf("Listing all files for %s/%s/pull/%d (Page %d)...", gr.Owner, gr.Name, pri, opts.Page)
		files, resp, err := githubClient.ListPullRequestFiles(ctx, gr.Owner, gr.Name, pri, opts)
		if err != nil {
			return fmt.Errorf("unable to list files for %s/%s/pull/%d (Page %d): %w", gr.Owner, gr.Name, pri, opts.Page, err)
		}

		if err = cb(files, resp); err != nil {
			return fmt.Errorf("callback failed for %s/%s/pull/%d (Page %d): %w", gr.Owner, gr.Name, pri, opts.Page, err)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return nil
}

// parseTestsFromFiles processes files and extracts test information
func (gr GithubRepo) parseTestsFromFiles(ctx context.Context, filesFiltered map[string]struct{}, pr *github.PullRequest, splitTestsAt string, githubClient gh.GitHubClientInterface, httpClient gh.HTTPClientInterface) (*map[string][]string, error) {
	serviceTestMap := map[string]map[string]bool{}
	testRegEx := regexp.MustCompile("func Test")

	clog.Log.Debugf("  parsing content:")
	for f := range filesFiltered {
		clog.Log.Debugf("    download %s", f)

		if pr.MergeCommitSHA == nil {
			return nil, fmt.Errorf("merge commit SHA is nil, is there a merge conflict?")
		}

		// Get file contents
		fileContents, _, _, err := githubClient.GetContents(ctx, gr.Owner, gr.Name, f, &github.RepositoryContentGetOptions{Ref: *pr.MergeCommitSHA})
		if err != nil {
			c.Printf("    <darkGray>FAILED to download %s</>\n", f)
		}

		if fileContents == nil {
			return nil, fmt.Errorf("downloading file (%s): no contents", f)
		}

		// GetContents has a 1MB limit. Use the DownloadURL to ensure we get full contents.
		if fileContents.DownloadURL == nil || *fileContents.DownloadURL == "" {
			return nil, fmt.Errorf("downloading file (%s): missing DownloadURL", f)
		}

		// Download file content
		resp, err := httpClient.Get(*fileContents.DownloadURL)
		if err != nil {
			return nil, fmt.Errorf("downloading file (%s): %w", f, err)
		}

		defer resp.Body.Close()

		// Parse tests from file
		tests, err := gr.parseTestsFromFileContent(resp.Body, testRegEx)
		if err != nil {
			return nil, fmt.Errorf("parsing tests from file (%s): %w", f, err)
		}

		// Extract service name from file path
		service := gr.extractServiceFromPath(f)

		// Add tests to service map
		gr.addTestsToServiceMap(serviceTestMap, service, tests, splitTestsAt)
	}

	return gr.convertServiceMapToSlices(serviceTestMap), nil
}

// parseTestsFromFileContent extracts test function names from file content
func (gr GithubRepo) parseTestsFromFileContent(body interface{ Read([]byte) (int, error) }, testRegEx *regexp.Regexp) ([]string, error) {
	var tests []string
	s := bufio.NewScanner(body)
	for s.Scan() {
		l := s.Text()

		if testRegEx.MatchString(l) {
			clog.Log.Tracef("found test line: %s", l)
			tests = append(tests, strings.Split(l, " ")[1]) // should always be true because test pattern is "func Test"
		}
	}

	if err := s.Err(); err != nil {
		return nil, fmt.Errorf("scanner error: %w", err)
	}

	return tests, nil
}

// extractServiceFromPath extracts service name from file path
func (gr GithubRepo) extractServiceFromPath(filePath string) string {
	service := ""
	parts := strings.Split(filePath, "/services/")
	if len(parts) == 2 {
		service = strings.Split(parts[1], "/")[0]
	}
	return service
}

// addTestsToServiceMap adds tests to the service test map
func (gr GithubRepo) addTestsToServiceMap(serviceTestMap map[string]map[string]bool, service string, tests []string, splitTestsAt string) {
	for _, t := range tests {
		clog.Log.Debugf("test: %s", t)

		if _, ok := serviceTestMap[service]; !ok {
			serviceTestMap[service] = make(map[string]bool)
		}

		// if there is nothing split on `(` to make sure we just get the full function name
		serviceTestMap[service][strings.Split(strings.Split(t, splitTestsAt)[0], "(")[0]] = true
	}
}

// convertServiceMapToSlices converts the service test map to the expected output format
func (gr GithubRepo) convertServiceMapToSlices(serviceTestMap map[string]map[string]bool) *map[string][]string {
	serviceTests := map[string][]string{}
	for service := range serviceTestMap {
		serviceTests[service] = []string{}
		for test := range serviceTestMap[service] {
			serviceInfo := ""
			if service != "" {
				serviceInfo = fmt.Sprintf("%s: ", service)
			}
			clog.Log.Debugf("%s%s", serviceInfo, test)
			serviceTests[service] = append(serviceTests[service], test)
		}
	}

	return &serviceTests
}

// ListAllPullRequestFiles uses dependency injection for testability
func (gr GithubRepo) ListAllPullRequestFiles(pri int, cb func([]*github.CommitFile, *github.Response) error) error {
	githubClient, _, ctx := gr.NewClientWithInterfaces()
	return gr.listAllPullRequestFilesWithClient(ctx, pri, githubClient, cb)
}

// GetAllPullRequestFiles uses dependency injection for testability
func (gr GithubRepo) GetAllPullRequestFiles(pri int, filterRegExStr string) (*map[string]struct{}, error) {
	githubClient, _, ctx := gr.NewClientWithInterfaces()
	return gr.getAllPullRequestFilesWithClient(ctx, pri, filterRegExStr, githubClient)
}
