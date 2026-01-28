package cli

import (
	"bufio"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/go-github/v45/github"
	c "github.com/gookit/color" //nolint:misspell
	"github.com/katbyte/tctest/lib/chttp"
	"github.com/katbyte/tctest/lib/clog"
	"github.com/pkg/browser"
)

// TODO reorg this file

func (f FlagData) GetPrTests(number int, title string) (*map[string][]string, error) {
	gr := f.NewRepo()

	prURL := gr.PrURL(number)
	c.Printf("Discovering tests for pr <cyan>#%d</> %s\n", number, title)
	c.Printf("  <darkGray>%s</>\n", prURL)
	serviceTests, err := gr.PrTests(number, f.GH.FileRegEx, f.GH.SplitTestsOn)

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

// todo break this apart - get/check PR state, get files, filter/process files, get tests, get services.
func (gr GithubRepo) PrTests(pri int, filterRegExStr, splitTestsAt string) (*map[string][]string, error) {
	client, ctx := gr.NewClient()
	httpClient := chttp.NewHTTPClient("HTTP")

	clog.Log.Debugf("fetching data for PR %s/%s/#%d...", gr.Owner, gr.Name, pri)
	pr, _, err := client.PullRequests.Get(ctx, gr.Owner, gr.Name, pri)
	if err != nil {
		return nil, err
	}

	clog.Log.Debugf("  checking pr state: %v", *pr.State)
	if pr.State != nil && *pr.State == "closed" {
		return nil, errors.New("cannot start build for a closed pr")
	}

	clog.Log.Tracef("listing files...")
	filesFiltered, err := gr.GetAllPullRequestFiles(pri, filterRegExStr)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR files for %s/%s/pull/%d: %w", gr.Owner, gr.Name, pri, err)
	}

	// for each file get content and parse out test files & services
	serviceTestMap := map[string]map[string]bool{}
	clog.Log.Debugf("  parsing content:")
	for f := range *filesFiltered {
		testRegEx := regexp.MustCompile("func Test")

		clog.Log.Debugf("    download %s", f)

		if pr.MergeCommitSHA == nil {
			return nil, errors.New("merge commit SHA is nil, is there a merge conflict?")
		}

		// DownloadContents always performs a directory listing for the file,
		// which has a 1000 file limit.
		fileContents, _, _, err := client.Repositories.GetContents(ctx, gr.Owner, gr.Name, f, &github.RepositoryContentGetOptions{Ref: *pr.MergeCommitSHA})
		if err != nil {
			c.Printf("    <darkGray>FAILED to download %s</>\n", f)
			continue
		}

		if fileContents == nil {
			return nil, fmt.Errorf("downloading file (%s): no contents", f)
		}

		// GetContents has a 1MB limit. Use the DownloadURL to ensure we get full contents.
		if fileContents.DownloadURL == nil || *fileContents.DownloadURL == "" {
			return nil, fmt.Errorf("downloading file (%s): missing DownloadURL", f)
		}

		// todo thread ctx
		//nolint: noctx
		resp, err := httpClient.Get(*fileContents.DownloadURL)
		if err != nil {
			return nil, fmt.Errorf("downloading file (%s): %w", f, err)
		}

		defer func() { _ = resp.Body.Close() }()

		var tests []string
		s := bufio.NewScanner(resp.Body)
		for s.Scan() {
			l := s.Text()

			if testRegEx.MatchString(l) {
				clog.Log.Tracef("found test line: %s", l)
				tests = append(tests, strings.Split(l, " ")[1]) // should always be true because test pattern is "func Test"
			}
		}

		if err := s.Err(); err != nil {
			fmt.Printf("pr file scanner error occurred: %s", err)
		}

		service := ""
		parts := strings.Split(f, "/services/")
		if len(parts) == 2 {
			service = strings.Split(parts[1], "/")[0]
		}

		for _, t := range tests {
			clog.Log.Debugf("test: %s", t)

			if _, ok := serviceTestMap[service]; !ok {
				serviceTestMap[service] = make(map[string]bool)
			}

			// if there is nothing split on `(` to make sure we just get the full function name
			serviceTestMap[service][strings.Split(strings.Split(t, splitTestsAt)[0], "(")[0]] = true
		}
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

func (gr GithubRepo) ListAllPullRequestFiles(pri int, cb func([]*github.CommitFile, *github.Response) error) error {
	client, ctx := gr.NewClient()

	opts := &github.ListOptions{
		Page:    1,
		PerPage: 100,
	}

	for {
		clog.Log.Debugf("Listing all files for %s/%s/pull/%d (Page %d)...", gr.Owner, gr.Name, pri, opts.Page)
		files, resp, err := client.PullRequests.ListFiles(ctx, gr.Owner, gr.Name, pri, opts)
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

func (gr GithubRepo) GetAllPullRequestFiles(pri int, filterRegExStr string) (*map[string]struct{}, error) {
	result := make(map[string]struct{})
	filterRegEx := regexp.MustCompile(filterRegExStr)

	err := gr.ListAllPullRequestFiles(pri, func(files []*github.CommitFile, _ *github.Response) error {
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

			f := strings.Replace(name, ".go", "_test.go", 1)
			result[f] = struct{}{}
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
