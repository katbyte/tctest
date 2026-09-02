// Package gh wraps the go-github client with the GitHub helpers (PRs, files, labels) shared between katbyte's tools.
package gh

import (
	"context"
	"strings"

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

	// APIURL and RawURL override the GitHub endpoints so tests can point at
	// mock servers; they also work for GitHub Enterprise. Empty means the
	// public github.com endpoints.
	APIURL string
	RawURL string

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

	if tok := t.Token; tok != nil {
		src := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: *tok},
		)
		httpClient = oauth2.NewClient(ctx, src)
	}

	httpClient.Transport = common.NewRetryTransport("GitHub", common.NewTransport("GitHub", httpClient.Transport), 3)

	client, err := github.NewClient(github.WithHTTPClient(httpClient))
	if err != nil {
		// only reachable if a client option fails, which WithHTTPClient never does
		clog.Log.Fatalf("creating github client: %v", err)
	}

	return client, ctx
}

// NewClient returns a github client for this repo, pointing at APIURL when set.
// It shadows the promoted Token.NewClient so all repo-scoped calls honour the override.
func (r Repo) NewClient() (*github.Client, context.Context) {
	if r.APIURL == "" {
		return r.Token.NewClient()
	}

	ctx := context.Background()
	httpClient := common.NewHTTPClient("GitHub")

	if t := r.Token.Token; t != nil {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: *t},
		)
		httpClient = oauth2.NewClient(ctx, ts)
	}

	httpClient.Transport = common.NewRetryTransport("GitHub", common.NewTransport("GitHub", httpClient.Transport), 3)

	apiURL := strings.TrimSuffix(r.APIURL, "/") + "/"
	client, err := github.NewClient(github.WithHTTPClient(httpClient), github.WithEnterpriseURLs(apiURL, apiURL))
	if err != nil {
		clog.Log.Errorf("creating github client with custom api url %q: %v", r.APIURL, err)
		return r.Token.NewClient()
	}

	return client, ctx
}
