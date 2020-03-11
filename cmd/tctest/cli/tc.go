package cli

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"syscall"
	"time"

	c "github.com/gookit/color"
	"github.com/katbyte/tctest/common"
	"golang.org/x/crypto/ssh/terminal"
)

func TcCmd(server, buildTypeId, branch, testRegEx, user, pass string, wait bool) error {
	c.Printf("triggering <magenta>%s</> for <darkGray>%s...</>\n", branch, testRegEx)
	c.Printf("  <darkGray>%s@%s#%s</>\n", user, server, buildTypeId)

	// prompt for password if not passed in somehow
	if pass == "" {
		fmt.Print("  password:")
		passBytes, err := terminal.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return fmt.Errorf("unable to read in password : %v", err)
		}
		pass = string(passBytes)
		fmt.Println("")
	}

	buildId, buildUrl, err := TcBuild(server, buildTypeId, branch, testRegEx, user, pass, wait)
	if err != nil {
		return fmt.Errorf("unable to trigger build: %v", err)
	}

	c.Printf("  build <green>%s</> queued: <darkGray>%s</>\n", buildId, buildUrl)

	if wait {
		err := waitForBuild(server, buildId, user, pass)
		if err != nil {
			return fmt.Errorf("error waiting for build %s to finish: %v", buildId, err)
		}
		err = TcTestResults(server, buildId, user, pass, wait)
		if err != nil {
			return fmt.Errorf("error printing results from build %s: %v", buildId, err)
		}
	}

	return nil
}

func TcBuild(server, buildTypeId, branch, testRegEx, user, pass string, wait bool) (string, string, error) {
	url := fmt.Sprintf("https://%s/app/rest/2018.1/buildQueue", server)
	body := fmt.Sprintf(`
<build>
	<buildType id="%s"/>
	<properties>
		<property name="BRANCH_NAME" value="%s"/>
		<property name="TEST_PATTERN" value="%s"/>
	</properties>
</build>
`, buildTypeId, branch, testRegEx)

	statusCode, body, err := makeTcApiCall(url, body, "POST", user, pass)
	if err != nil {
		return "", "", fmt.Errorf("error creating build request: %v", err)
	}

	if statusCode != http.StatusOK {
		return "", "", fmt.Errorf("HTTP status NOT OK: %d", statusCode)
	}

	data := struct {
		BuildId string `xml:"id,attr"`
	}{}
	if err := xml.NewDecoder(strings.NewReader(body)).Decode(&data); err != nil {
		return "", "", fmt.Errorf("unable to decode XML: %d", statusCode)
	}

	return data.BuildId, fmt.Sprintf("https://%s/viewQueued.html?itemId=%s", server, data.BuildId), nil
}

func TcTestResults(server, buildId, user, pass string, wait bool) error {
	url := fmt.Sprintf("https://%s/app/rest/2018.1/builds/%s/state", server, buildId)
	statusCode, buildStatus, err := makeTcApiCall(url, "", "GET", user, pass)
	if err != nil {
		return fmt.Errorf("error looking for build %s state: %v", buildId, err)
	}
	if statusCode == http.StatusNotFound {
		return fmt.Errorf("no build ID %s found in running builds or queue", buildId)
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("HTTP status NOT OK: %d", statusCode)
	}

	if buildStatus != "finished" && wait {
		err := waitForBuild(server, buildId, user, pass)
		if err != nil {
			return fmt.Errorf("error waiting for build %s to finish: %v", buildId, err)
		}
	}

	url = fmt.Sprintf("https://%s/downloadBuildLog.html?buildId=%s", server, buildId)
	statusCode, body, err := makeTcApiCall(url, "", "GET", user, pass)
	if err != nil {
		return fmt.Errorf("error looking for build %s results: %v", buildId, err)
	}

	if statusCode == http.StatusNotFound {
		// Possibly a queued build, check for it
		url := fmt.Sprintf("https://%s/app/rest/2018.1/buildQueue/id:%s", server, buildId)
		statusCode, _, err := makeTcApiCall(url, "", "GET", user, pass)

		if err != nil {
			return fmt.Errorf("error checking for build %s in queue: %v", buildId, err)
		}

		if statusCode == http.StatusNotFound {
			return fmt.Errorf("no build ID %s found in running builds or queue", buildId)
		}
		if statusCode != http.StatusOK {
			return fmt.Errorf("HTTP status NOT OK: %d", statusCode)
		}
		return fmt.Errorf("build %s still queued, check results later...", buildId)
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("HTTP status NOT OK: %d", statusCode)
	}

	r := regexp.MustCompile(`^\s*--- (FAIL|PASS|SKIP):`)
	for _, line := range strings.Split(body, "\n") {
		if r.MatchString(line) {
			fmt.Printf("%s\n", line)
		}
	}

	if buildStatus == "running" && !wait {
		// If we didn't want to wait and it's not finished, print a warning at the end so people notice it
		return fmt.Errorf("build %s is still running, test results may be incomplete!", buildId)
	}

	return nil
}

func makeTcApiCall(url, body, method, user, pass string) (int, string, error) {
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		return 0, "", fmt.Errorf("building http request for url %s failed: %v", url, err)
	}

	req.SetBasicAuth(user, pass)
	req.Header.Set("Content-Type", "application/xml")

	resp, err := common.Http.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("http request failed: %v", err)
	}
	defer resp.Body.Close()

	// The calling function will figure out what to do with these
	// because e.g. sometimes a 404 is an error, but sometimes it just means something might be queued
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, "", fmt.Errorf("error reading response body: %v", err)
	}
	return resp.StatusCode, string(b), nil
}

func waitForBuild(server, buildId, user, pass string) error {
	fmt.Printf("Waiting for build %s status to be 'finished'...\n", buildId)
	url := fmt.Sprintf("https://%s/app/rest/2018.1/builds/%s/state", server, buildId)

	// At some point we might want these to be user configurable
	queueTimeTimeout := 60
	runningTimeTimout := 60

	var queueTime, runningTime int
	for {
		if runningTime > runningTimeTimout {
			return fmt.Errorf("timeout waiting for build %s to become finished (running for %d minutes)", buildId, runningTimeTimout)
		}
		if queueTime > queueTimeTimeout {
			return fmt.Errorf("timeout waiting for build %s to start running (queued for %d minutes)", buildId, queueTimeTimeout)
		}

		statusCode, body, err := makeTcApiCall(url, "", "GET", user, pass)
		if err != nil {
			return err
		}
		if statusCode == http.StatusNotFound {
			return fmt.Errorf("no build ID %s found in running builds or queue", buildId)
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
