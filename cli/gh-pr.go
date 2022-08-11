package cli

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/go-github/v45/github"
	c "github.com/gookit/color" //nolint:misspell
	"github.com/katbyte/tctest/lib/common"
	"github.com/pkg/browser"
)

// TODO reorg this file

func (f FlagData) GetPrTests(pr int) (*map[string][]string, error) {
	gr := f.NewRepo()

	prURL := gr.PrURL(pr)
	c.Printf("Discovering tests for pr <cyan>#%d</> <darkGray>(%s)...</>\n", pr, prURL)
	serviceTests, err := gr.PrTests(pr, f.GH.FileRegEx, f.GH.SplitTestsOn)
	if err != nil {
		return nil, fmt.Errorf("pr list failed: %w", err)
	}

	for service, tests := range *serviceTests {
		c.Printf("    <yellow>%s</>:\n", service)
		for _, t := range tests {
			c.Printf("      %s\n", t)
		}
	}

	if f.OpenInBrowser {
		if err := browser.OpenURL(prURL); err != nil {
			c.Printf("failed to open build %s in browser", prURL)
		}
	}

	return serviceTests, nil
}

// todo break this apart - get/check PR state, get files, filter/process files, get tests, get services.
func (gr githubRepo) PrTests(pri int, filterRegExStr, splitTestsAt string) (*map[string][]string, error) {
	client, ctx := gr.NewClient()
	httpClient := common.NewHTTPClient("HTTP")
	fileRegEx := regexp.MustCompile(filterRegExStr)

	common.Log.Debugf("fetching data for PR %s/%s/#%d...", gr.Owner, gr.Repo, pri)
	pr, _, err := client.PullRequests.Get(ctx, gr.Owner, gr.Name, pri)
	if err != nil {
		return nil, err
	}

	common.Log.Debugf("  checking pr state: %v", *pr.State)
	if pr.State != nil && *pr.State == "closed" {
		return nil, fmt.Errorf("cannot start build for a closed pr")
	}

	common.Log.Tracef("listing files...")
	files, _, err := client.PullRequests.ListFiles(ctx, gr.Owner, gr.Name, pri, nil)
	if err != nil {
		return nil, err
	}

	// filter out uninteresting files and convert non test files to test files and only retain unique
	filesFiltered := map[string]bool{}
	common.Log.Debugf("  filtering files (%s)", filterRegExStr)
	for _, f := range files {
		if f.Filename == nil {
			continue
		}

		name := *f.Filename
		common.Log.Debugf("    %v", *f.Filename)

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
			filesFiltered[name] = true
			continue
		}

		if !fileRegEx.MatchString(name) {
			continue
		}

		f := strings.Replace(name, ".go", "_test.go", 1)
		filesFiltered[f] = true
	}
	common.Log.Debugf("  FOUND %d", len(filesFiltered))

	if len(filesFiltered) == 0 {
		return nil, fmt.Errorf("found no files matching: %s", filterRegExStr)
	}
	// log.Println(files) TODO debug message here

	// for each file get content and parse out test files & services
	serviceTestMap := map[string]map[string]bool{}
	common.Log.Debugf("  parsing content:")
	for f := range filesFiltered {
		testRegEx := regexp.MustCompile("func Test")

		common.Log.Debugf("    download %s", f)

		if pr.MergeCommitSHA == nil {
			return nil, fmt.Errorf("merge commit SHA is nil, is there a merge conflict?")
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
		// nolint: noctx
		resp, err := httpClient.Get(*fileContents.DownloadURL)
		if err != nil {
			return nil, fmt.Errorf("downloading file (%s): %w", f, err)
		}

		defer resp.Body.Close()

		var tests []string
		s := bufio.NewScanner(resp.Body)
		for s.Scan() {
			l := s.Text()

			if testRegEx.MatchString(l) {
				common.Log.Tracef("found test line: %s", l)
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
			common.Log.Debugf("test: %s", t)

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
				serviceInfo = fmt.Sprintf("%s: ", service)
			}
			common.Log.Debugf("%s%s", serviceInfo, test)
			serviceTests[service] = append(serviceTests[service], test)
		}
	}

	return &serviceTests, nil
}
