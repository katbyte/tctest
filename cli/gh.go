package cli

import (
	"strings"

	common2 "github.com/katbyte/tctest/lib/common"
	"github.com/katbyte/tctest/lib/gh"
)

// wrap the common gh lib shared with my other tools. splits common GH code from this CLI tool's specific tooling code
type githubRepo struct {
	gh.Repo
}

func (f FlagData) NewRepo() githubRepo {
	ownerrepo := f.GH.Repo

	parts := strings.Split(ownerrepo, "/")
	if len(parts) != 2 {
		panic("repo was not in the format owner/repo") // this is bad but works for now
	}
	owner, repo := parts[0], parts[1]

	token := f.GH.Token
	common2.Log.Debugf("new gh: %s@%s/%s", token, owner, repo)

	return githubRepo{gh.NewRepo(owner, repo, token)}
}
