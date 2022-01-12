package cli

import (
	"context"
	"strings"

	"github.com/google/go-github/v41/github"
	"github.com/katbyte/tctest/common"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
)

type GithubRepo struct {
	Owner string
	Repo  string
	Token *string
}

func NewGithubRepoFromViper() GithubRepo {
	ownerrepo := viper.GetString("repo")

	parts := strings.Split(ownerrepo, "/")
	if len(parts) != 2 {
		panic("repo was not in the format owner/repo") // this is bad but works for now
	}
	owner, repo := parts[0], parts[1]

	token := viper.GetString("token-gh")
	common.Log.Debugf("new gh: %s@%s/%s", token, owner, repo)
	return NewGithubRepo(owner, repo, token)
}

func NewGithubRepo(owner, repo, token string) GithubRepo {
	r := GithubRepo{
		Owner: owner,
		Repo:  repo,
		Token: nil,
	}

	if token != "" {
		r.Token = &token
	}

	return r
}

func (gr GithubRepo) NewClient() (*github.Client, context.Context) {
	ctx := context.Background()
	httpClient := common.NewHTTPClient("GitHub")

	if gr.Token != nil {
		t := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: *gr.Token},
		)
		httpClient = oauth2.NewClient(ctx, t)
		httpClient.Transport = common.NewTransport("GitHub", httpClient.Transport)
	}

	return github.NewClient(httpClient), ctx
}
