package cli

import (
	"github.com/katbyte/tctest/lib/tc"
)

// wrap the common gh lib shared with my other tools. splits common GH code from this CLI tool's specific tooling code
type tcServer struct {
	tc.Server
}

func (f FlagData) NewServer() tcServer {
	return tcServer{tc.NewServer(f.TC.ServerURL, f.TC.Token, f.TC.User, f.TC.Pass)}
}

// Forwarding methods to help older golangci-lint with embedded type method promotion
func (ts tcServer) RunBuild(buildTypeID, buildProperties, branch string, testRegEx string, skipQueue bool) (int, string, error) {
	return ts.Server.RunBuild(buildTypeID, buildProperties, branch, testRegEx, skipQueue)
}

func (ts tcServer) AddLabels(buildID int, labels []string) error {
	return ts.Server.AddLabels(buildID, labels)
}

func (ts tcServer) WaitForBuild(buildID int, queueTimeout, runTimeout int) error {
	return ts.Server.WaitForBuild(buildID, queueTimeout, runTimeout)
}

func (ts tcServer) BuildState(buildID int) (int, string, error) {
	return ts.Server.BuildState(buildID)
}

func (ts tcServer) BuildLog(buildID int) (int, string, error) {
	return ts.Server.BuildLog(buildID)
}

func (ts tcServer) CheckBuildLogStatus(statusCode int, buildID int) error {
	return ts.Server.CheckBuildLogStatus(statusCode, buildID)
}

func (ts tcServer) GetBuildsForPR(buildTypeID string, pr int, latest, wait bool, queueTimeout, runTimeout int) (*[]tc.Build, error) {
	return ts.Server.GetBuildsForPR(buildTypeID, pr, latest, wait, queueTimeout, runTimeout)
}
