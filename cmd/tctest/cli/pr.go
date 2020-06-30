package cli

import (
	"bufio"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/go-github/github"
	//nolint:misspell
	c "github.com/gookit/color"
	"github.com/katbyte/tctest/common"
)

func PrUrl(repo string, pr int) string {
	return "https://github.com/" + repo + "/pull/" + strconv.Itoa(pr)
}

func PrCmd(ownerrepo string, pr int, fileRegExStr, splitTestsAt string, servicePackagesMode bool) (*[]string, error) {
	parts := strings.Split(ownerrepo, "/")
	owner, repo := parts[0], parts[1]

	c.Printf("Discovering tests for pr <cyan>#%d</> <darkGray>(%s)...</>\n", pr, PrUrl(repo, pr))
	tests, err := PrTests(owner, repo, pr, fileRegExStr, splitTestsAt, servicePackagesMode)
	if err != nil {
		return nil, fmt.Errorf("pr list failed: %v", err)
	}

	for _, t := range *tests {
		fmt.Printf("    %s\n", t)
	}
	return tests, nil
}

func PrTests(owner, repo string, pri int, fileRegExStr, splitTestsAt string, servicePackagesMode bool) (*[]string, error) {
	ctx := context.Background()
	client := github.NewClient(nil)
	fileRegEx := regexp.MustCompile(fileRegExStr)

	common.Log.Debugf("fetching data for PR %s/%s/#%d...", owner, repo, pri)
	pr, _, err := client.PullRequests.Get(ctx, owner, repo, pri)
	if err != nil {
		return nil, err
	}

	common.Log.Debugf("  checking pr state: %v", *pr.State)
	if pr.State != nil && *pr.State == "closed" {
		return nil, fmt.Errorf("cannot start build for a closed pr")
	}

	common.Log.Tracef("listing files...")
	files, _, err := client.PullRequests.ListFiles(ctx, owner, repo, pri, nil)
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

		reader, err := client.Repositories.DownloadContents(ctx, owner, repo, f, &github.RepositoryContentGetOptions{Ref: *pr.MergeCommitSHA})
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
