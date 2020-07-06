package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/github"
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
		panic(fmt.Sprint("repo was not in the format owner/repo")) // this is bad but works for now
	}
	owner, repo := parts[0], parts[1]

	token := viper.GetString("token-gh")
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

	if gr.Token != nil {
		t := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: *gr.Token},
		)
		return github.NewClient(oauth2.NewClient(ctx, t)), ctx
	}

	return github.NewClient(nil), ctx
}
