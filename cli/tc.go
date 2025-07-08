package cli

import (
	"github.com/katbyte/tctest/lib/tc"
)

// wrap the common gh lib shared with my other tools. splits common GH code from this CLI tool's specific tooling code
type TcServer struct {
	tc.Server
}

func (f FlagData) NewServer() TcServer {
	return TcServer{tc.NewServer(f.TC.ServerURL, f.TC.Token, f.TC.User, f.TC.Pass)}
}

// Forwarding methods to help older golangci-lint with embedded type method promotion
func (ts TcServer) RunBuild(buildTypeID, buildProperties, branch string, testRegEx string, skipQueue bool) (int, string, error) {
	return ts.Server.RunBuild(buildTypeID, buildProperties, branch, testRegEx, skipQueue)
}

func (ts TcServer) AddLabels(buildID int, labels []string) error {
	return ts.Server.AddLabels(buildID, labels)
}

func (ts TcServer) WaitForBuild(buildID int, queueTimeout, runTimeout int) error {
	return ts.Server.WaitForBuild(buildID, queueTimeout, runTimeout)
}

func (ts TcServer) BuildState(buildID int) (int, string, error) {
	return ts.Server.BuildState(buildID)
}

func (ts TcServer) BuildLog(buildID int) (int, string, error) {
	return ts.Server.BuildLog(buildID)
}

func (ts TcServer) CheckBuildLogStatus(statusCode int, buildID int) error {
	return ts.Server.CheckBuildLogStatus(statusCode, buildID)
}

func (ts TcServer) GetBuildsForPR(buildTypeID string, pr int, latest, wait bool, queueTimeout, runTimeout int) (*[]tc.Build, error) {
	return ts.Server.GetBuildsForPR(buildTypeID, pr, latest, wait, queueTimeout, runTimeout)
}
