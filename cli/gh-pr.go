package cli

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
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

	if pr.MergeCommitSHA == nil {
		return nil, errors.New("merge commit SHA is nil, is there a merge conflict?")
	}

	// for each file get content and parse out test files & services
	serviceTestMap := map[string]map[string]bool{}
	clog.Log.Debugf("  parsing content:")
	for f := range *filesFiltered {
		clog.Log.Debugf("    download %s", f)

		// DownloadContents always performs a directory listing for the file,
		// which has a 1000 file limit.
		fileContents, _, _, err := client.Repositories.GetContents(ctx, gr.Owner, gr.Name, f, &github.RepositoryContentGetOptions{Ref: *pr.MergeCommitSHA})
		if err != nil {
			clog.Log.Debugf("    skipping %s (not found at merge commit)", f)
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

		content, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("reading file (%s): %w", f, err)
		}

		// use go/ast to extract test function names
		var tests []string
		fset := token.NewFileSet()
		parsed, parseErr := parser.ParseFile(fset, f, content, 0)
		if parseErr != nil {
			clog.Log.Debugf("    failed to parse %s, falling back to regex: %v", f, parseErr)
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

	// track resource files that need sibling test file discovery
	// key: directory path, value: list of resource prefixes (e.g. "foo_resource")
	resourceDirs := map[string][]string{}

	// track changed files and test files for output
	var changedFiles []string
	var testFiles []string
	changedTestFiles := map[string]bool{} // tracks which test files came from the PR diff
	derivedTestFiles := map[string]bool{} // tracks which test files were derived
	testFileSeen := map[string]bool{}     // dedup test files

	err := gr.ListAllPullRequestFiles(pri, func(files []*github.CommitFile, _ *github.Response) error {
		for _, f := range files {
			if f.Filename == nil {
				continue
			}

			name := *f.Filename
			clog.Log.Debugf("    %v (%s)", name, f.GetStatus())

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

			if !filterRegEx.MatchString(name) {
				continue
			}

			changedFiles = append(changedFiles, name)

			// derive the primary test file
			testFile := strings.Replace(name, ".go", "_test.go", 1)
			result[testFile] = struct{}{}
			derivedTestFiles[testFile] = true
			if !testFileSeen[testFile] {
				testFiles = append(testFiles, testFile)
				testFileSeen[testFile] = true
			}

			// for resource files, note the directory and prefix so we can
			// discover all related test files (e.g. _list_test.go, _identity_gen_test.go)
			if strings.HasSuffix(name, "_resource.go") {
				dir := name[:strings.LastIndex(name, "/")]
				base := name[strings.LastIndex(name, "/")+1:]
				prefix := strings.TrimSuffix(base, ".go") // e.g. "foo_resource"
				resourceDirs[dir] = append(resourceDirs[dir], prefix)
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get all files for %s/%s/pull/%d: %w", gr.Owner, gr.Name, pri, err)
	}

	// for each directory containing a modified resource file, list all files
	// and find sibling test files matching the resource prefix
	if len(resourceDirs) > 0 {
		client, ctx := gr.NewClient()
		for dir, prefixes := range resourceDirs {
			clog.Log.Debugf("  listing directory %s for related test files...", dir)
			_, dirContents, _, err := client.Repositories.GetContents(ctx, gr.Owner, gr.Name, dir, nil)
			if err != nil {
				clog.Log.Debugf("  failed to list directory %s: %v", dir, err)
				continue
			}

			for _, entry := range dirContents {
				entryName := entry.GetName()
				if !strings.HasSuffix(entryName, "_test.go") {
					continue
				}
				for _, prefix := range prefixes {
					if strings.HasPrefix(entryName, prefix) {
						fullPath := dir + "/" + entryName
						if _, exists := result[fullPath]; !exists {
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
			}
		}
	}

	// print file regex and changed files
	c.Printf("  file regex: <darkGray>%s</>\n", filterRegExStr)
	c.Printf("  changed files (<yellow>%d</>):\n", len(changedFiles))
	for _, f := range changedFiles {
		dir := f[:strings.LastIndex(f, "/")+1]
		base := f[strings.LastIndex(f, "/")+1:]
		if strings.HasSuffix(f, "_test.go") {
			c.Printf("    <darkGray>%s</><fg=28>%s</>\n", dir, base)
		} else {
			c.Printf("    <darkGray>%s%s</>\n", dir, base)
		}
	}

	// print test files
	c.Printf("  test files (<yellow>%d</>):\n", len(testFiles))
	for _, f := range testFiles {
		dir := f[:strings.LastIndex(f, "/")+1]
		base := f[strings.LastIndex(f, "/")+1:]

		// build label based on whether file is changed, derived, or both
		var labels []string
		if changedTestFiles[f] {
			labels = append(labels, "CHANGED")
		}
		if derivedTestFiles[f] {
			labels = append(labels, "DERIVED")
		}
		label := strings.Join(labels, "/")

		c.Printf("    <darkGray>%s</><fg=28>%s</> <darkGray>[%s]</>\n", dir, base, label)
	}

	clog.Log.Debugf("  FOUND %d", len(result))
	for f := range result {
		clog.Log.Debugf("     %s", f)
	}

	return &result, nil
}
