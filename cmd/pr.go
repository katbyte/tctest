package cmd

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"

	"github.com/katbyte/tctest/common"
)

func PrUrl(repo, pr string) string {
	return "https://github.com/" + repo + "/pull/" + pr
}

func PrCmd(repo, pr, fileRegExStr, splitTestsAt string) (*[]string, error) {
	//PrListCmd(pr, flags.PR)
	// todo COLOUR
	fmt.Printf("Discovering tests for pr #%s (%s)...\n", pr, PrUrl(repo, pr))
	tests, err := PrTests(repo, pr, fileRegExStr, splitTestsAt)
	if err != nil {
		return nil, fmt.Errorf("pr list failed: %v", err)
	}

	for _, t := range *tests {
		fmt.Printf("    %s\n", t)
	}
	return tests, nil
}

func PrTests(repo, pr, fileRegExStr, splitTestsAt string) (*[]string, error) {
	fileRegEx := regexp.MustCompile(fileRegExStr)

	sha, err := PrMergeCommit(repo, pr)
	if err != nil {
		return nil, fmt.Errorf("error getting pr merge commit sha1: %v", err)
	}

	files, err := PrFiles(repo, pr)
	if err != nil {
		return nil, fmt.Errorf("error getting pr files: %v", err)
	}

	// filter out uninteresting files and
	// convert non test files to test files and only retain unique
	filesm := map[string]bool{}
	for _, f := range files {
		if !fileRegEx.MatchString(f) {
			continue
		}

		if !strings.HasSuffix(f, "_test.go") {
			filesm[strings.Replace(f, ".go", "_test.go", 1)] = true
		} else {
			filesm[f] = true
		}
	}

	if len(filesm) <= 0 {
		return nil, fmt.Errorf("found no files matching: %s", fileRegExStr)
	}
	// log.Println(files) TODO debug message here

	// for each file get content and parse out test files
	testsm := map[string]bool{}
	for f := range filesm {
		tests, err := PrFileTests(repo, sha, f)
		if err != nil {
			return nil, fmt.Errorf("Error fetching tests: %v", err)
		}

		for _, t := range tests {
			testsm[strings.Split(t, splitTestsAt)[0]] = true
		}
	}

	tests := []string{}
	for k := range testsm {
		// log.Println(url) TODO trace message here
		tests = append(tests, k)
	}

	return &tests, nil
}

func PrMergeCommit(repo, pr string) (string, error) {
	url := "https://api.github.com/repos/" + repo + "/pulls/" + pr
	json := struct {
		MergeCommitSha string `json:"merge_commit_sha"`
	}{}

	// get the mege commit SHA to look at for file content
	if err := common.HttpUnmarshalJson(url, &json); err != nil {
		return "", fmt.Errorf("error getting merge commit SHA: %v", err)
	}

	if json.MergeCommitSha == "" {
		return "", fmt.Errorf("unable to find merge_commit_sha @ %s", url)
	}

	// log.Println(sha) TODO debug message here
	return json.MergeCommitSha, nil
}

func PrFiles(repo, pr string) ([]string, error) {
	url := "https://api.github.com/repos/" + repo + "/pulls/" + pr + "/files"
	var json []struct {
		FileName string `json:"filename"`
	}

	// get the list of files for the PR
	if err := common.HttpUnmarshalJson(url, &json); err != nil {
		return nil, fmt.Errorf("error getting file list: %v", err)
	}
	// log.Println(files) TODO debug message here for number of files

	var files []string
	for _, v := range json {
		files = append(files, v.FileName)
		//todo trace message here
	}

	return files, nil
}

func PrFileTests(repo, sha, file string) ([]string, error) {
	testRegEx := regexp.MustCompile("func Test")
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", repo, sha, file)

	// get file content
	// log.Println(url) TODO debug message here
	reader, err := common.HttpGetReader(url)
	if err != nil {
		return nil, fmt.Errorf("unable to get content from %s: %v", url, err)
	}

	// find test lines
	var tests []string
	s := bufio.NewScanner(*reader)
	for s.Scan() {
		l := s.Text()

		// log.Println(url) TODO trace message here
		if testRegEx.MatchString(l) {
			// log.Println(url) TODO debug message here
			tests = append(tests, strings.Split(l, " ")[1]) //should always be true because test pattern is "func Test"
		}
	}

	if err := s.Err(); err != nil {
		fmt.Printf("pr file scanner error occured: %s", err)
	}

	return tests, nil
}
