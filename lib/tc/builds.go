package tc

import (
	"encoding/xml"
	"fmt"
	"net/http"
	//nolint:misspell
)

// TODO TODO TODO
// build id needs to become int everywhere

type BuildsResp struct {
	XMLName xml.Name          `xml:"builds"`
	Builds  []BuildsRespBuild `xml:"build"`
}

type BuildsRespBuild struct {
	XMLName    xml.Name `xml:"build"`
	ID         string   `xml:"id,attr"`
	Number     string   `xml:"number,attr"`
	State      string   `xml:"state,attr"`
	BranchName string   `xml:"branchName,attr"`
	WebURL     string   `xml:"webUrl,attr"`
}

func (s Server) BuildLocator(buildTypeId string, pr int, latest, wait bool, queueTimeout, runTimeout int) (*[]BuildsRespBuild, error) {

	queryArgs := fmt.Sprintf("buildType:%s,branch:name:refs/pull/%d/merge,running:any", buildTypeId, pr)
	if latest {
		queryArgs += ",count:1"
	}

	statusCode, body, err := s.makeGetRequest(fmt.Sprintf("/app/rest/2018.1/builds?locator=%s", queryArgs))
	if statusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no build for PR %s found in running builds or queue", pr)
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status NOT OK: %d", statusCode)
	}
	if body == "" {
		return nil, fmt.Errorf("empty xml file of builds for PR %s", pr)
	}

	buildLocatorResults := []byte(body)
	var tcb BuildsResp
	err = xml.Unmarshal(buildLocatorResults, &tcb)
	if err != nil {
		return nil, err
	}
	if len(tcb.Builds) == 0 {
		return nil, fmt.Errorf("no builds parsed from XML response")
	}

	for _, build := range tcb.Builds {
		if build.State != "finished" && wait {
			err := s.WaitForBuild(build.ID, queueTimeout, runTimeout)
			if err != nil {
				return nil, fmt.Errorf("error waiting for PR %s, build %s to finish: %w", pr, build.ID, err)
			}
		}
	}

	return &tcb.Builds, nil
}
