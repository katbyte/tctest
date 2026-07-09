package cli

import (
	"strings"

	"github.com/katbyte/tctest/lib/clog"
	"github.com/katbyte/tctest/lib/gh"
)

// wrap the common gh lib shared with my other tools. splits common GH code from this CLI tool's specific tooling code
type GithubRepo struct {
	gh.Repo
}

func (f FlagData) NewRepo() GithubRepo {
	ownerrepo := f.GH.Repo

	parts := strings.Split(ownerrepo, "/")
	if len(parts) != 2 {
		panic("repo was not in the format owner/repo")
	}
	owner, repo := parts[0], parts[1]

	token := f.GH.Token
	clog.Log.Debugf("new gh: %s@%s/%s", maskToken(token), owner, repo)

	return GithubRepo{gh.NewRepo(owner, repo, token)}
}

func maskToken(token string) string {
	if len(token) <= 4 {
		return "****"
	}
	return token[:4] + "****"
}
