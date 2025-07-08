package cli

import (
	"context"
	"strings"

	"github.com/google/go-github/v45/github"
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
		panic("repo was not in the format owner/repo") // this is bad but works for now
	}
	owner, repo := parts[0], parts[1]

	token := f.GH.Token
	clog.Log.Debugf("new gh: %s@%s/%s", token, owner, repo)

	return GithubRepo{gh.NewRepo(owner, repo, token)}
}

// Forwarding methods to help older golangci-lint with embedded type method promotion
func (gr GithubRepo) PrURL(pr int) string {
	return gr.Repo.PrURL(pr)
}

func (gr GithubRepo) GetAllPullRequests(state string) (*[]github.PullRequest, error) {
	return gr.Repo.GetAllPullRequests(state)
}

func (gr GithubRepo) NewClient() (*github.Client, context.Context) {
	return gr.Repo.NewClient()
}
