package tc

import (
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

type buildsResp struct {
	XMLName xml.Name          `xml:"builds"`
	Builds  []buildsRespBuild `xml:"build"`
}

type buildsRespBuild struct {
	XMLName    xml.Name `xml:"build"`
	ID         string   `xml:"id,attr"`
	Number     string   `xml:"number,attr"`
	State      string   `xml:"state,attr"`
	BranchName string   `xml:"branchName,attr"`
	WebURL     string   `xml:"webUrl,attr"`
}

type Build struct {
	ID     int
	Number int
	Branch string
	URL    string
	State  string
}

func (s Server) GetBuildsForPR(buildTypeID string, pr int, latest, wait bool, queueTimeout, runTimeout int) (*[]Build, error) {
	queryArgs := fmt.Sprintf("buildType:%s,branch:name:refs/pull/%d/merge,running:any", buildTypeID, pr)
	if latest {
		queryArgs += ",count:1"
	}

	statusCode, body, err := s.makeGetRequest("/app/rest/2018.1/builds?locator=" + queryArgs)
	if err != nil {
		return nil, fmt.Errorf("unable to list builds (%s): %w", queryArgs, err)
	}
	if statusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no build for PR %d found in running builds or queue", pr)
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status NOT OK: %d", statusCode)
	}
	if body == "" {
		return nil, fmt.Errorf("empty xml file of builds for PR %d", pr)
	}

	buildLocatorResults := []byte(body)
	var tcb buildsResp
	err = xml.Unmarshal(buildLocatorResults, &tcb)
	if err != nil {
		return nil, err
	}
	if len(tcb.Builds) == 0 {
		return nil, errors.New("no builds parsed from XML response")
	}

	builds := []Build{}
	for _, build := range tcb.Builds {
		b := Build{
			Branch: build.BranchName,
			URL:    build.WebURL,
			State:  build.State,
		}

		b.ID, err = strconv.Atoi(build.ID)
		if err != nil {
			return nil, fmt.Errorf("unable to convert build.ID (%s) from response into an integer: %w", build.ID, err)
		}

		b.Number, err = strconv.Atoi(build.Number)
		if err != nil {
			return nil, fmt.Errorf("unable to convert build.Number (%s) from response into an integer: %w", build.Number, err)
		}

		if build.State != "finished" && wait {
			err := s.WaitForBuild(b.ID, queueTimeout, runTimeout)
			if err != nil {
				return nil, fmt.Errorf("error waiting for PR %d, build %d to finish: %w", pr, b.ID, err)
			}
		}

		builds = append(builds, b)
	}

	return &builds, nil
}
