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
