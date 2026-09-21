package tc

import (
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

// ErrNotQueued is returned by CancelQueuedBuild when the build has left the queue since it was listed, e.g. because it
// started running or someone else already removed it.
var ErrNotQueued = errors.New("build is no longer queued")

// queuePageSize is how many queued builds are requested per page; the queue is paged explicitly so a very large queue
// is never silently truncated by a server-side default limit
const queuePageSize = 1000

// queueFields selects just what is needed to identify and display a queued build; the default response omits the
// build configuration's name and project path, which are what the queue commands display and match against
const queueFields = "build(id,buildTypeId,branchName,webUrl,buildType(name,projectName))"

type queueResp struct {
	XMLName xml.Name         `xml:"builds"`
	Builds  []queueRespBuild `xml:"build"`
}

type queueRespBuild struct {
	ID          int    `xml:"id,attr"`
	BuildTypeID string `xml:"buildTypeId,attr"`
	BranchName  string `xml:"branchName,attr"`
	WebURL      string `xml:"webUrl,attr"`
	BuildType   struct {
		Name        string `xml:"name,attr"`
		ProjectName string `xml:"projectName,attr"`
	} `xml:"buildType"`
}

// QueuedBuild is a build waiting in the TeamCity build queue.
type QueuedBuild struct {
	ID          int
	BuildTypeID string
	Name        string // the build configuration's name, e.g. "Service Sweeper"
	ProjectName string // the full project path, e.g. "Development / Mau's Project / Google Beta / Nightly Tests"
	Branch      string // empty when the build is on the default branch of a configuration without branches
	URL         string
}

// FullName returns the build configuration's full path as the TeamCity UI shows it in the queue,
// e.g. "Development / Mau's Project / Google Beta / Nightly Tests / Service Sweeper".
func (b QueuedBuild) FullName() string {
	name := b.Name
	if name == "" {
		name = b.BuildTypeID
	}
	if b.ProjectName == "" {
		return name
	}
	return b.ProjectName + " / " + name
}

// GetBuildQueue returns every build in the queue that the authenticated user can see, in queue order.
func (s Server) GetBuildQueue() ([]QueuedBuild, error) {
	return s.getBuildQueuePaged(queuePageSize)
}

func (s Server) getBuildQueuePaged(pageSize int) ([]QueuedBuild, error) {
	var builds []QueuedBuild
	seen := map[int]bool{}

	for start := 0; ; start += pageSize {
		q := url.Values{}
		q.Set("locator", fmt.Sprintf("start:%d,count:%d", start, pageSize))
		q.Set("fields", queueFields)

		statusCode, body, err := s.makeGetRequest("/app/rest/" + s.apiVersion() + "/buildQueue?" + q.Encode())
		if err != nil {
			return nil, fmt.Errorf("unable to list build queue: %w", err)
		}
		if statusCode != http.StatusOK {
			if details := condenseBody(body); details != "" {
				return nil, fmt.Errorf("HTTP status NOT OK: %d listing build queue: %s", statusCode, details)
			}
			return nil, fmt.Errorf("HTTP status NOT OK: %d listing build queue", statusCode)
		}

		var page queueResp
		if err := xml.Unmarshal([]byte(body), &page); err != nil {
			return nil, fmt.Errorf("unable to decode build queue XML: %w", err)
		}

		for _, b := range page.Builds {
			// the queue keeps moving between page requests, which can shift a build onto the next page as well
			if seen[b.ID] {
				continue
			}
			seen[b.ID] = true

			builds = append(builds, QueuedBuild{
				ID:          b.ID,
				BuildTypeID: b.BuildTypeID,
				Name:        b.BuildType.Name,
				ProjectName: b.BuildType.ProjectName,
				Branch:      b.BranchName,
				URL:         b.WebURL,
			})
		}

		if len(page.Builds) < pageSize {
			return builds, nil
		}
	}
}

// CancelQueuedBuild removes a build from the queue, recording comment as the reason in its build history. It returns
// an error wrapping ErrNotQueued when the build has already left the queue.
func (s Server) CancelQueuedBuild(buildID int, comment string) error {
	body := fmt.Sprintf(`<buildCancelRequest comment='%s' readdIntoQueue='false'/>`, xmlEscape(comment))

	statusCode, respBody, err := s.makePostRequestWithXMLContentType(fmt.Sprintf("/app/rest/%s/buildQueue/id:%d", s.apiVersion(), buildID), body)
	if err != nil {
		return fmt.Errorf("error removing build %d from the queue: %w", buildID, err)
	}
	if statusCode == http.StatusOK {
		return nil
	}

	// TeamCity reports a build that already left the queue with a not found or bad request depending on where it went,
	// so rather than guess from the status code, check whether the build is actually still queued
	stateCode, state, stateErr := s.BuildState(buildID)
	if stateErr == nil {
		switch {
		case stateCode == http.StatusNotFound:
			return fmt.Errorf("build %d no longer exists: %w", buildID, ErrNotQueued)
		case stateCode == http.StatusOK && state != "queued":
			return fmt.Errorf("build %d is %s: %w", buildID, state, ErrNotQueued)
		}
	}

	if details := condenseBody(respBody); details != "" {
		return fmt.Errorf("HTTP status NOT OK: %d removing build %d from the queue: %s", statusCode, buildID, details)
	}
	return fmt.Errorf("HTTP status NOT OK: %d removing build %d from the queue", statusCode, buildID)
}
