package cli

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	//nolint:misspell
	c "github.com/gookit/color"
	common2 "github.com/katbyte/tctest/lib/common"
	"github.com/pkg/browser"
	"github.com/spf13/viper"
)

type TeamCity struct {
	server   string
	token    *string
	username *string
	password *string
}

type TCBuilds struct {
	XMLName xml.Name  `xml:"builds"`
	Builds  []TCBuild `xml:"build"`
}

type TCBuild struct {
	XMLName    xml.Name `xml:"build"`
	ID         string   `xml:"id,attr"`
	Number     string   `xml:"number,attr"`
	State      string   `xml:"state,attr"`
	BranchName string   `xml:"branchName,attr"`
	WebURL     string   `xml:"webUrl,attr"`
}

func NewTeamCityFromViper() TeamCity {
	server := viper.GetString("server")
	token := viper.GetString("token-tc")
	password := viper.GetString("password")
	username := viper.GetString("username")

	return NewTeamCity(server, token, username, password)
}

func NewTeamCity(server, token, username, password string) TeamCity {
	if token != "" {
		return NewTeamCityUsingTokenAuth(server, token)
	}

	if username != "" {
		return NewTeamCityUsingBasicAuth(server, username, password)
	}

	// should probably do something better here
	panic("token & username are both empty")
}

func NewTeamCityUsingTokenAuth(server, token string) TeamCity {
	common2.Log.Debugf("new tc: %s@%s", token, server)
	return TeamCity{
		server: server,
		token:  &token,
	}
}

func NewTeamCityUsingBasicAuth(server, username, password string) TeamCity {
	common2.Log.Debugf("new tc: %s@%s", username, server)
	return TeamCity{
		server:   server,
		username: &username,
		password: &password,
	}
}

func (tc TeamCity) BuildCmd(buildTypeID, buildProperties, branch, testRegex, serviceInfo string, wait bool, skipQueue bool, open bool) error {
	c.Printf("triggering <magenta>%s</>%s @ <darkGray>%s...</>\n", branch, serviceInfo, buildTypeID)

	buildID, buildURL, err := tc.runBuild(buildTypeID, buildProperties, branch, testRegex, skipQueue)
	if err != nil {
		return fmt.Errorf("unable to trigger build: %w", err)
	}

	c.Printf("  build <green>%s</> queued: <darkGray>%s</> with <darkGray>%s</>\n", buildID, buildURL, testRegex)

	if open {
		if err := browser.OpenURL(buildURL); err != nil {
			c.Printf("failed to open build %s in browser", buildID)
		}
	}

	if wait {
		common2.Log.Debugf("waiting...")
		err := tc.waitForBuild(buildID)
		if err != nil {
			return fmt.Errorf("error waiting for build %s to finish: %w", buildID, err)
		}
		err = tc.TestResultsCmd(buildID, wait)
		if err != nil {
			return fmt.Errorf("error printing results from build %s: %w", buildID, err)
		}
	}

	return nil
}

func (tc TeamCity) runBuild(buildTypeID, buildProperties, branch string, testRegEx string, skipQueue bool) (string, string, error) {
	common2.Log.Debugf("triggering build for %q", buildTypeID)
	statusCode, body, err := tc.triggerBuild(buildTypeID, branch, testRegEx, buildProperties, skipQueue)
	if err != nil {
		return "", "", fmt.Errorf("error creating build request: %w", err)
	}

	if statusCode != http.StatusOK {
		return "", "", fmt.Errorf("HTTP status NOT OK: %d", statusCode)
	}

	data := struct {
		BuildID string `xml:"id,attr"`
	}{}

	if err := xml.NewDecoder(strings.NewReader(body)).Decode(&data); err != nil {
		return "", "", fmt.Errorf("unable to decode XML: %d", statusCode)
	}

	return data.BuildID, fmt.Sprintf("https://%s/viewQueued.html?itemId=%s", tc.server, data.BuildID), nil
}

func (tc TeamCity) TestResultsCmd(buildID string, wait bool) error {
	statusCode, buildStatus, err := tc.buildState(buildID)
	if err != nil {
		return fmt.Errorf("error looking for build %s state: %w", buildID, err)
	}
	if statusCode == http.StatusNotFound {
		return fmt.Errorf("no build ID %s found in running builds or queue", buildID)
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("HTTP status NOT OK: %d", statusCode)
	}

	if buildStatus != "finished" && wait {
		err := tc.waitForBuild(buildID)
		if err != nil {
			return fmt.Errorf("error waiting for build %s to finish: %w", buildID, err)
		}
	}

	statusCode, body, err := tc.buildLog(buildID)
	if err != nil {
		return fmt.Errorf("error looking for build %s results: %w", buildID, err)
	}

	if err := tc.checkBuildLogStatus(statusCode, buildID); err != nil {
		return err
	}

	outputTestResults(body)

	if buildStatus == "running" && !wait {
		// If we didn't want to wait and it's not finished, print a warning at the end so people notice it
		return fmt.Errorf("build %s is still running, test results may be incomplete", buildID)
	}

	return nil
}

func (tc TeamCity) TestResultsByPRCmd(pr, buildTypeID string, latest, wait bool) error {
	locatorParams := fmt.Sprintf("buildType:%s,branch:name:refs/pull/%s/merge,running:any", buildTypeID, pr)
	if latest {
		locatorParams += ",count:1"
	}

	statusCode, respBody, err := tc.buildLocator(locatorParams)
	if err != nil {
		return fmt.Errorf("error looking for builds for PR %s state: %w", pr, err)
	}
	if statusCode == http.StatusNotFound {
		return fmt.Errorf("no build for PR %s found in running builds or queue", pr)
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("HTTP status NOT OK: %d", statusCode)
	}
	if respBody == "" {
		return fmt.Errorf("empty xml file of builds for PR %s", pr)
	}

	buildLocatorResults := []byte(respBody)
	var tcb TCBuilds
	err = xml.Unmarshal(buildLocatorResults, &tcb)
	if err != nil {
		return err
	}
	if len(tcb.Builds) == 0 {
		return fmt.Errorf("no builds parsed from XML response")
	}

	for _, build := range tcb.Builds {
		if build.State != "finished" && wait {
			err := tc.waitForBuild(build.ID)
			if err != nil {
				return fmt.Errorf("error waiting for PR %s, build %s to finish: %w", pr, build.ID, err)
			}
		}
	}

	for _, build := range tcb.Builds {
		statusCode, body, err := tc.buildLog(build.ID)
		if err != nil {
			return fmt.Errorf("error looking for PR %s, build %s results: %w", pr, build.ID, err)
		}

		if err := tc.checkBuildLogStatus(statusCode, build.ID); err != nil {
			return err
		}

		fmt.Printf("Test Results (buildID: %s, buildNumber: %s, branch: %s):\n", build.ID, build.Number, build.BranchName)
		outputTestResults(body)

		if build.State == "running" && !wait {
			// If we didn't want to wait and it's not finished, print a warning at the end so people notice it
			fmt.Printf("[WARN] build (ID: %s) for PR %s is still running, test results may be incomplete\n", build.ID, pr)
		}

		fmt.Printf("Build Log: %s\n\n", build.WebURL)
	}

	return nil
}

func (tc TeamCity) buildLog(buildID string) (int, string, error) {
	return tc.makeGetRequest(fmt.Sprintf("/downloadBuildLog.html?buildID=%s", buildID))
}

func (tc TeamCity) buildQueue(buildID string) (int, string, error) {
	return tc.makeGetRequest(fmt.Sprintf("/app/rest/2018.1/buildQueue/id:%s", buildID))
}

func (tc TeamCity) buildState(buildID string) (int, string, error) {
	return tc.makeGetRequest(fmt.Sprintf("/app/rest/2018.1/builds/%s/state", buildID))
}

func (tc TeamCity) buildLocator(queryArgs string) (int, string, error) {
	return tc.makeGetRequest(fmt.Sprintf("/app/rest/2018.1/builds?locator=%s", queryArgs))
}

func (tc TeamCity) waitForBuild(buildID string) error {
	fmt.Printf("Waiting for build %s status to be 'finished'...\n", buildID)

	// At some point we might want these to be user configurable
	queueTimeTimeout := 60
	runningTimeTimout := 60

	var queueTime, runningTime int
	for {
		if runningTime > runningTimeTimout {
			return fmt.Errorf("timeout waiting for build %s to become finished (running for %d minutes)", buildID, runningTimeTimout)
		}
		if queueTime > queueTimeTimeout {
			return fmt.Errorf("timeout waiting for build %s to start running (queued for %d minutes)", buildID, queueTimeTimeout)
		}

		statusCode, body, err := tc.buildState(buildID)
		if err != nil {
			return err
		}
		if statusCode == http.StatusNotFound {
			return fmt.Errorf("no build ID %s found in running builds or queue", buildID)
		}
		if statusCode != http.StatusOK {
			return fmt.Errorf("HTTP status NOT OK: %d", statusCode)
		}
		if body == "queued" {
			queueTime++ // We track this separately since things might be queued for a while due to other tests, sweepers, etc
		}

		if body == "running" {
			runningTime++
		}

		if body == "finished" {
			return nil
		}

		time.Sleep(1 * time.Minute)
	}
}

func (tc TeamCity) triggerBuild(buildTypeID, branch string, testPattern, buildProperties string, skipQueue bool) (int, string, error) {
	bodyAdditionalProperties := ""

	if buildProperties != "" {
		common2.Log.Debugf("adding additional properties:")

		for _, p := range strings.Split(buildProperties, ";") {
			parts := strings.Split(p, "=")
			if len(parts) != 2 {
				return 0, "", fmt.Errorf("unable to parse build property '%s': missing =", p)
			}

			common2.Log.Debugf("  property:%s=%s", parts[0], parts[1])
			bodyAdditionalProperties += fmt.Sprintf("\t\t<property name=\"%s\" value=\"%s\"/>\n", parts[0], parts[1])
		}
	}

	// for now we have two types of build - historical providers (BRANCH_NAME & TEST_PATTERN), new azurerm (teamcity.build.branch, TEST_PREFIX)
	// should be safe to send both
	body := fmt.Sprintf(`
<build>
	<triggeringOptions queueAtTop="%[5]s"/>
	<buildType id="%[1]s"/>
	<properties>
        <property name="teamcity.build.branch" value="%[2]s"/>
		<property name="BRANCH_NAME" value="%[2]s"/>
		<property name="TEST_PATTERN" value="%[3]s"/>
        <property name="TEST_PREFIX" value="%[3]s"/>
%[4]s	</properties>
</build>
`, buildTypeID, branch, testPattern, bodyAdditionalProperties, strconv.FormatBool(skipQueue))

	return tc.makePostRequest("/app/rest/2018.1/buildQueue", body)
}

func (tc TeamCity) makeGetRequest(endpoint string) (int, string, error) {
	uri := fmt.Sprintf("https://%s%s", tc.server, endpoint)
	req, err := http.NewRequestWithContext(context.Background(), "GET", uri, nil)
	if err != nil {
		return 0, "", fmt.Errorf("building http request for url %s failed: %w", uri, err)
	}

	return tc.performRequest(req)
}

func (tc TeamCity) makePostRequest(endpoint, body string) (int, string, error) {
	uri := fmt.Sprintf("https://%s%s", tc.server, endpoint)
	req, err := http.NewRequestWithContext(context.Background(), "POST", uri, strings.NewReader(body))
	if err != nil {
		return 0, "", fmt.Errorf("building http request for url %s failed: %w", uri, err)
	}

	return tc.performRequest(req)
}

func (tc TeamCity) performRequest(req *http.Request) (int, string, error) {
	if tc.token != nil {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *tc.token))
	} else {
		req.SetBasicAuth(*tc.username, *tc.password)
	}

	req.Header.Set("Content-Type", "application/xml")

	resp, err := common2.HTTP.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	// The calling function will figure out what to do with these
	// because e.g. sometimes a 404 is an error, but sometimes it just means something might be queued
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, "", fmt.Errorf("error reading response body: %w", err)
	}

	return resp.StatusCode, string(b), nil
}

func (tc TeamCity) checkBuildLogStatus(statusCode int, buildID string) error {
	if statusCode == http.StatusNotFound {
		// Possibly a queued build, check for it
		statusCode, _, err := tc.buildQueue(buildID)
		if err != nil {
			return fmt.Errorf("error checking for build %s in queue: %w", buildID, err)
		}

		if statusCode == http.StatusNotFound {
			return fmt.Errorf("no build ID %s found in running builds or queue", buildID)
		}
		if statusCode != http.StatusOK {
			return fmt.Errorf("HTTP status NOT OK: %d", statusCode)
		}

		return fmt.Errorf("build %s still queued, check results later", buildID)
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("HTTP status NOT OK: %d", statusCode)
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
