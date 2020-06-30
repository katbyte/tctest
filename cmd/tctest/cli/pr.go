package cli

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/go-github/github"

	//nolint:misspell
	c "github.com/gookit/color"
	"github.com/katbyte/tctest/common"
)

func (gr GithubRepo) PrUrl(pr int) string {
	return "https://github.com/" + gr.Owner + "/" + gr.Repo + "/pull/" + strconv.Itoa(pr)
}

func (gr GithubRepo) PrCmd(pr int, fileRegExStr, splitTestsAt string, servicePackagesMode bool) (*[]string, error) {

	c.Printf("Discovering tests for pr <cyan>#%d</> <darkGray>(%s)...</>\n", pr, gr.PrUrl(pr))
	tests, err := gr.PrTests(pr, fileRegExStr, splitTestsAt, servicePackagesMode)
	if err != nil {
		return nil, fmt.Errorf("pr list failed: %v", err)
	}

	for _, t := range *tests {
		fmt.Printf("    %s\n", t)
	}
	return tests, nil
}

func (gr GithubRepo) PrTests(pri int, fileRegExStr, splitTestsAt string, servicePackagesMode bool) (*[]string, error) {
	client, ctx := gr.NewClient()
	fileRegEx := regexp.MustCompile(fileRegExStr)

	common.Log.Debugf("fetching data for PR %s/%s/#%d...", gr.Owner, gr.Repo, pri)
	pr, _, err := client.PullRequests.Get(ctx, gr.Owner, gr.Repo, pri)
	if err != nil {
		return nil, err
	}

	common.Log.Debugf("  checking pr state: %v", *pr.State)
	if pr.State != nil && *pr.State == "closed" {
		return nil, fmt.Errorf("cannot start build for a closed pr")
	}

	common.Log.Tracef("listing files...")
	files, _, err := client.PullRequests.ListFiles(ctx, gr.Owner, gr.Repo, pri, nil)
	if err != nil {
		return nil, err
	}

	// filter out uninteresting files and convert non test files to test files and only retain unique
	filesFiltered := map[string]bool{}
	for _, f := range files {
		if f.Filename == nil {
			continue
		}

		name := *f.Filename
		common.Log.Debugf("  checking file %v", *f.Filename)

		if strings.HasSuffix(name, "_test.go") {
			filesFiltered[name] = true
			continue
		}

		if !fileRegEx.MatchString(name) {
			continue
		}

		f := strings.Replace(name, ".go", "_test.go", 1)

		if servicePackagesMode {
			i := strings.LastIndex(f, "/")
			filesFiltered[f[:i]+"/tests"+f[i:]] = true
		} else {
			filesFiltered[f] = true
		}
	}

	if len(filesFiltered) == 0 {
		return nil, fmt.Errorf("found no files matching: %s", fileRegExStr)
	}
	// log.Println(files) TODO debug message here

	// for each file get content and parse out test files
	testsm := map[string]bool{}
	for f := range filesFiltered {
		testRegEx := regexp.MustCompile("func Test")

		reader, err := client.Repositories.DownloadContents(ctx, gr.Owner, gr.Repo, f, &github.RepositoryContentGetOptions{Ref: *pr.MergeCommitSHA})
		if err != nil {
			return nil, err
		}

		var tests []string
		s := bufio.NewScanner(reader)
		for s.Scan() {
			l := s.Text()

			if testRegEx.MatchString(l) {
				common.Log.Tracef("found test line: %s", l)
				tests = append(tests, strings.Split(l, " ")[1]) //should always be true because test pattern is "func Test"
			}
		}

		if err := s.Err(); err != nil {
			fmt.Printf("pr file scanner error occurred: %s", err)
		}

		for _, t := range tests {
			common.Log.Debugf("test: %s", t)
			// if there is nothing split on `(` to make sure we just get the full function name
			testsm[strings.Split(strings.Split(t, splitTestsAt)[0], "(")[0]] = true
		}
	}

	tests := []string{}
	for k := range testsm {
		common.Log.Debugf("test prefix: %s", k)
		tests = append(tests, k)
	}

	return &tests, nil
}
