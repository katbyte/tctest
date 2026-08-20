package tc

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/katbyte/tctest/lib/clog"
	"github.com/katbyte/tctest/lib/cout"
)

func (s Server) RunBuild(buildTypeID, buildProperties, branch, testRegEx string, skipQueue bool) (buildID int, buildURL string, err error) {
	clog.Log.Debugf("triggering build for %q", buildTypeID)
	statusCode, body, err := s.TriggerBuild(buildTypeID, branch, testRegEx, buildProperties, skipQueue)
	if err != nil {
		return 0, "", fmt.Errorf("error creating build request: %w", err)
	}

	if statusCode != http.StatusOK {
		if details := condenseBody(body); details != "" {
			return 0, "", fmt.Errorf("HTTP status NOT OK: %d for build type %q: %s", statusCode, buildTypeID, details)
		}
		return 0, "", fmt.Errorf("HTTP status NOT OK: %d for build type %q", statusCode, buildTypeID)
	}

	data := struct {
		BuildID string `xml:"id,attr"`
	}{}

	if err := xml.NewDecoder(strings.NewReader(body)).Decode(&data); err != nil {
		return 0, "", fmt.Errorf("unable to decode XML (status %d): %w", statusCode, err)
	}

	bid, err := strconv.Atoi(data.BuildID)
	if err != nil {
		return 0, "", fmt.Errorf("unable to convert build.ID (%s) from response into an integer: %w", data.BuildID, err)
	}

	return bid, fmt.Sprintf("https://%s/viewQueued.html?itemId=%d", s.Server, bid), nil
}

// TriggerBuild queues a TeamCity build for the given build type and branch with the test pattern and additional properties.
// todo is there any reason to not inline this into runbuild?
func (s Server) TriggerBuild(buildTypeID, branch, testPattern, buildProperties string, skipQueue bool) (statusCode int, respBody string, err error) {
	var additionalProps strings.Builder

	if buildProperties != "" {
		clog.Log.Debugf("adding additional properties:")

		for p := range strings.SplitSeq(buildProperties, ";") {
			// SplitN so values may themselves contain '=' (base64, -run=Foo, URLs)
			parts := strings.SplitN(p, "=", 2)
			if len(parts) != 2 {
				return 0, "", fmt.Errorf("unable to parse build property '%s': expected KEY=VALUE", p)
			}

			clog.Log.Debugf("  property:%s=%s", parts[0], parts[1])
			fmt.Fprintf(&additionalProps, "\t\t<property name=\"%s\" value=\"%s\"/>\n", xmlEscape(parts[0]), xmlEscape(parts[1]))
		}
	}

	bodyAdditionalProperties := additionalProps.String()

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
`, xmlEscape(buildTypeID), xmlEscape(branch), xmlEscape(testPattern), bodyAdditionalProperties, strconv.FormatBool(skipQueue))

	return s.makePostRequestWithXMLContentType("/app/rest/2018.1/buildQueue", body)
}

// condenseBody flattens a TeamCity error response body onto a single line so it can be included in an error message;
// bodies are multiline (e.g. "Responding with error, status code: 404 (Not Found).\nDetails: ...NotFoundException:
// No build type nor template is found by id 'X'.") and can trail off into long request dumps, so cap the length
func condenseBody(body string) string {
	const maxLen = 400
	body = strings.Join(strings.Fields(body), " ")
	if len(body) > maxLen {
		body = body[:maxLen] + "..."
	}
	return body
}

// xmlEscape escapes a string for use in an XML attribute value; regexes and
// property values routinely contain &, <, and " which would break the request body
func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s)) // only fails on writer errors, which strings.Builder never returns
	return b.String()
}

func (s Server) BuildLog(buildID int) (statusCode int, body string, err error) {
	return s.makeGetRequest(fmt.Sprintf("/downloadBuildLog.html?buildId=%d", buildID))
}

func (s Server) BuildQueue(buildID int) (statusCode int, body string, err error) {
	return s.makeGetRequest(fmt.Sprintf("/app/rest/2018.1/buildQueue/id:%d", buildID))
}

func (s Server) BuildState(buildID int) (statusCode int, body string, err error) {
	return s.makeGetRequest(fmt.Sprintf("/app/rest/2018.1/builds/%d/state", buildID))
}

func (s Server) WaitForBuild(buildID, queueTimeout, runTimeout int) error {
	cout.Printf("Waiting for build %d status to be 'finished'...\n", buildID)

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
			// 'finished' alone isn't success — cancelled builds (e.g. TeamCity hitting a temporary VCS error like
			// "cannot find commit") also report state 'finished', and would otherwise look like a clean run with no results
			return s.CheckFinishedBuild(buildID)
		}

		time.Sleep(1 * time.Minute)
	}
}

// finishedBuildResp is the subset of a build's detail needed to tell a cancelled build apart from one that ran
type finishedBuildResp struct {
	XMLName      xml.Name `xml:"build"`
	Status       string   `xml:"status,attr"`
	StatusText   string   `xml:"statusText"`
	CanceledInfo *struct {
		Text string `xml:"text"`
	} `xml:"canceledInfo"`
}

// CheckFinishedBuild returns an error if a finished build was cancelled instead of run to completion, including
// TeamCity's cancellation reason when it provides one. Cancelled builds carry a canceledInfo element and status
// UNKNOWN, either of which is treated as cancellation.
func (s Server) CheckFinishedBuild(buildID int) error {
	statusCode, body, err := s.makeGetRequest(fmt.Sprintf("/app/rest/2018.1/builds/%d", buildID))
	if err != nil {
		return fmt.Errorf("error fetching status for build %d: %w", buildID, err)
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("HTTP status NOT OK: %d fetching status for build %d", statusCode, buildID)
	}

	var build finishedBuildResp
	if err := xml.Unmarshal([]byte(body), &build); err != nil {
		return fmt.Errorf("unable to decode XML for build %d: %w", buildID, err)
	}

	if build.CanceledInfo == nil && build.Status != "UNKNOWN" {
		return nil
	}

	reason := ""
	if build.CanceledInfo != nil {
		reason = build.CanceledInfo.Text
	}
	if reason == "" {
		reason = build.StatusText
	}
	if reason == "" {
		return fmt.Errorf("build %d was cancelled", buildID)
	}
	return fmt.Errorf("build %d was cancelled: %s", buildID, reason)
}

func (s Server) CheckBuildLogStatus(statusCode, buildID int) error {
	if statusCode == http.StatusNotFound {
		// Possibly a queued build, check for it
		queueStatusCode, _, err := s.BuildQueue(buildID)
		if err != nil {
			return fmt.Errorf("error checking for build %d in queue: %w", buildID, err)
		}

		if queueStatusCode == http.StatusNotFound {
			return fmt.Errorf("no build ID %d found in running builds or queue", buildID)
		}
		if queueStatusCode != http.StatusOK {
			return fmt.Errorf("HTTP status NOT OK: %d", queueStatusCode)
		}

		return fmt.Errorf("build %d still queued, check results later", buildID)
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("HTTP status NOT OK: %d", statusCode)
	}

	return nil
}

// AddTags adds Tags to a TeamCity build run using the REST API
func (s Server) AddTags(buildID int, tags []string) error {
	if len(tags) == 0 {
		return nil // Nothing to do
	}

	clog.Log.Debugf("adding tags %v to build %d", tags, buildID)

	for _, tag := range tags {
		if tag == "" {
			return fmt.Errorf("received an empty string to add as tag to build %d", buildID)
		}

		// TeamCity REST API expects a simple text body for adding tags
		statusCode, _, err := s.makePostRequestWithContentType(fmt.Sprintf("/app/rest/2018.1/builds/id:%d/tags", buildID), tag, "text/plain")
		if err != nil {
			return fmt.Errorf("error adding tag '%s' to build %d: %w", tag, buildID, err)
		}

		if statusCode != http.StatusOK {
			return fmt.Errorf("HTTP status NOT OK when adding tag '%s' to build %d: %d", tag, buildID, statusCode)
		}

		clog.Log.Debugf("added tag '%s' to build %d", tag, buildID)
	}

	return nil
}
