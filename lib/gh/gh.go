// Package gh wraps the go-github client with the GitHub helpers (PRs, files, labels) shared between katbyte's tools.
package gh

import (
	"context"

	"github.com/google/go-github/v89/github"
	common "github.com/katbyte/tctest/lib/chttp"
	"github.com/katbyte/tctest/lib/clog"
	"golang.org/x/oauth2"
)

type Token struct {
	Token *string
}

type Repo struct {
	Owner string
	Name  string
	Token
}

func NewRepo(owner, repo, token string) Repo {
	r := Repo{
		Owner: owner,
		Name:  repo,
		Token: Token{
			Token: nil,
		},
	}

	if token != "" {
		r.Token.Token = &token
	}

	return r
}

func (t Token) NewClient() (*github.Client, context.Context) {
	ctx := context.Background()
	httpClient := common.NewHTTPClient("GitHub")

	if t := t.Token; t != nil {
		t := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: *t},
		)
		httpClient = oauth2.NewClient(ctx, t)
	}

	httpClient.Transport = common.NewRetryTransport("GitHub", common.NewTransport("GitHub", httpClient.Transport), 3)

	client, err := github.NewClient(github.WithHTTPClient(httpClient))
	if err != nil {
		// only reachable if a client option fails, which WithHTTPClient never does
		clog.Log.Fatalf("creating github client: %v", err)
	}

	return client, ctx
}
