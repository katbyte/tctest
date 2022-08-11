package cli

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	//nolint:misspell
	c "github.com/gookit/color"
	common2 "github.com/katbyte/tctest/lib/common"
	"github.com/pkg/browser"
)

func (f FlagData) BuildCmd(buildTypeID, branch, testRegex, service string) error {
	tc := f.NewServer()

	c.Printf("triggering <magenta>%s</>%s @ <darkGray>%s...</>\n", branch, service, buildTypeID)

	buildID, buildURL, err := tc.RunBuild(buildTypeID, f.TC.Build.Parameters, branch, testRegex, f.TC.Build.SkipQueue)
	if err != nil {
		return fmt.Errorf("unable to trigger build: %w", err)
	}

	c.Printf("  build <green>%d</> queued: <darkGray>%s</> with <darkGray>%s</>\n", buildID, buildURL, testRegex)

	if f.OpenInBrowser {
		if err := browser.OpenURL(buildURL); err != nil {
			c.Printf("failed to open build %d in browser", buildID)
		}
	}

	if f.TC.Build.Wait {
		common2.Log.Debugf("waiting...")
		err := tc.WaitForBuild(buildID, f.TC.Build.QueueTimeout, f.TC.Build.RunTimeout)
		if err != nil {
			return fmt.Errorf("error waiting for build %d to finish: %w", buildID, err)
		}
		err = f.BuildResultsCmd(buildID)
		if err != nil {
			return fmt.Errorf("error printing results from build %d: %w", buildID, err)
		}
	}

	return nil
}

func (f FlagData) BuildResultsCmd(buildID int) error {
	tc := f.NewServer()

	statusCode, buildStatus, err := tc.BuildState(buildID)
	if err != nil {
		return fmt.Errorf("error looking for build %d state: %w", buildID, err)
	}
	if statusCode == http.StatusNotFound {
		return fmt.Errorf("no build ID %d found in running builds or queue", buildID)
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("HTTP status NOT OK: %d", statusCode)
	}

	if buildStatus != "finished" && f.TC.Build.Wait {
		err := tc.WaitForBuild(buildID, f.TC.Build.QueueTimeout, f.TC.Build.RunTimeout)
		if err != nil {
			return fmt.Errorf("error waiting for build %d to finish: %w", buildID, err)
		}
	}

	statusCode, body, err := tc.BuildLog(buildID)
	if err != nil {
		return fmt.Errorf("error looking for build %d results: %w", buildID, err)
	}

	if err := tc.CheckBuildLogStatus(statusCode, buildID); err != nil {
		return err
	}

	outputTestResults(body)

	if buildStatus == "running" && !f.TC.Build.Wait {
		// If we didn't want to wait, and it's not finished, print a warning at the end so people notice it
		return fmt.Errorf("build %d is still running, test results may be incomplete", buildID)
	}

	return nil
}

func (f FlagData) BuildResultsForPRCmd(pr int) error {
	tc := f.NewServer()

	builds, err := tc.GetBuildsForPR(f.TC.Build.TypeID, pr, f.TC.Build.Latest, f.TC.Build.Wait, f.TC.Build.RunTimeout, f.TC.Build.RunTimeout)
	if err != nil {
		return fmt.Errorf("error looking for builds for PR %d state: %w", pr, err)
	}

	for _, build := range *builds {
		statusCode, body, err := tc.BuildLog(build.ID)
		if err != nil {
			return fmt.Errorf("error looking for PR %d, build %d results: %w", pr, build.ID, err)
		}

		if err := tc.CheckBuildLogStatus(statusCode, build.ID); err != nil {
			return err
		}

		fmt.Printf("Test Results (buildID: %d, buildNumber: %d, branch: %s):\n", build.ID, build.Number, build.Branch)
		outputTestResults(body)

		if build.State == "running" && !f.TC.Build.Wait {
			// If we didn't want to wait, and it's not finished, print a warning at the end so people notice it
			fmt.Printf("[WARN] build (ID: %d) for PR %d is still running, test results may be incomplete\n", build.ID, pr)
		}

		fmt.Printf("Build Log: %s\n\n", build.URL)
	}

	return nil
}

func outputTestResults(body string) {
	r := regexp.MustCompile(`^\s*--- (FAIL|PASS|SKIP):`)
	for _, line := range strings.Split(body, "\n") {
		if r.MatchString(line) {
			fmt.Printf("%s\n", line)
		}
	}
}
