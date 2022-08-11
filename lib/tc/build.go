package tc

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/katbyte/tctest/lib/common"
)

func (s Server) RunBuild(buildTypeID, buildProperties, branch string, testRegEx string, skipQueue bool) (int, string, error) {
	common.Log.Debugf("triggering build for %q", buildTypeID)
	statusCode, body, err := s.TriggerBuild(buildTypeID, branch, testRegEx, buildProperties, skipQueue)
	if err != nil {
		return 0, "", fmt.Errorf("error creating build request: %w", err)
	}

	if statusCode != http.StatusOK {
		return 0, "", fmt.Errorf("HTTP status NOT OK: %d", statusCode)
	}

	data := struct {
		BuildID string `xml:"id,attr"`
	}{}

	if err := xml.NewDecoder(strings.NewReader(body)).Decode(&data); err != nil {
		return 0, "", fmt.Errorf("unable to decode XML: %d", statusCode)
	}

	bid, err := strconv.Atoi(data.BuildID)
	if err != nil {
		return 0, "", fmt.Errorf("unable to convert build.ID (%d) from response into an integer: %w", bid, err)
	}

	return bid, fmt.Sprintf("https://%s/viewQueued.html?itemId=%d", s.Server, bid), nil
}

// todo is there any reason to not inline this into runbuild?
func (s Server) TriggerBuild(buildTypeID, branch string, testPattern, buildProperties string, skipQueue bool) (int, string, error) {
	bodyAdditionalProperties := ""

	if buildProperties != "" {
		common.Log.Debugf("adding additional properties:")

		for _, p := range strings.Split(buildProperties, ";") {
			parts := strings.Split(p, "=")
			if len(parts) != 2 {
				return 0, "", fmt.Errorf("unable to parse build property '%s': missing =", p)
			}

			common.Log.Debugf("  property:%s=%s", parts[0], parts[1])
			bodyAdditionalProperties += fmt.Sprintf("\t\t<property name=\"%s\" value=\"%s\"/>\n", parts[0], parts[1])
		}
	}

	// for now, we have two types of build - historical providers (BRANCH_NAME & TEST_PATTERN), new azurerm (teamcity.build.branch, TEST_PREFIX)
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

	return s.makePostRequest("/app/rest/2018.1/buildQueue", body)
}

func (s Server) BuildLog(buildID int) (int, string, error) {
	return s.makeGetRequest(fmt.Sprintf("/downloadBuildLog.html?buildID=%d", buildID))
}

func (s Server) BuildQueue(buildID int) (int, string, error) {
	return s.makeGetRequest(fmt.Sprintf("/app/rest/2018.1/buildQueue/id:%d", buildID))
}

func (s Server) BuildState(buildID int) (int, string, error) {
	return s.makeGetRequest(fmt.Sprintf("/app/rest/2018.1/builds/%d/state", buildID))
}

func (s Server) WaitForBuild(buildID int, queueTimeout, runTimeout int) error {
	fmt.Printf("Waiting for build %d status to be 'finished'...\n", buildID)

	var queueTime, runningTime int
	for {
		if runningTime > runTimeout {
			return fmt.Errorf("timeout waiting for build %d to become finished (running for %d minutes)", buildID, runTimeout)
		}
		if queueTime > queueTimeout {
			return fmt.Errorf("timeout waiting for build %d to start running (queued for %d minutes)", buildID, queueTimeout)
		}

		statusCode, body, err := s.BuildState(buildID)
		if err != nil {
			return err
		}
		if statusCode == http.StatusNotFound {
			return fmt.Errorf("no build ID %d found in running builds or queue", buildID)
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

func (s Server) CheckBuildLogStatus(statusCode int, buildID int) error {
	if statusCode == http.StatusNotFound {
		// Possibly a queued build, check for it
		statusCode, _, err := s.BuildQueue(buildID)
		if err != nil {
			return fmt.Errorf("error checking for build %d in queue: %w", buildID, err)
		}

		if statusCode == http.StatusNotFound {
			return fmt.Errorf("no build ID %d found in running builds or queue", buildID)
		}
		if statusCode != http.StatusOK {
			return fmt.Errorf("HTTP status NOT OK: %d", statusCode)
		}

		return fmt.Errorf("build %d still queued, check results later", buildID)
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("HTTP status NOT OK: %d", statusCode)
	}

	return nil
}
