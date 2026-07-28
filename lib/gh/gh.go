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

func (t Token) NewClient(opts ...github.ClientOptionsFunc) (*github.Client, context.Context) {
	ctx := context.Background()
	httpClient := common.NewHTTPClient("GitHub")

	if t := t.Token; t != nil {
		t := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: *t},
		)
		httpClient = oauth2.NewClient(ctx, t)
	}

	httpClient.Transport = common.NewRetryTransport("GitHub", common.NewTransport("GitHub", httpClient.Transport), 3)

	client, err := github.NewClient(append([]github.ClientOptionsFunc{github.WithHTTPClient(httpClient)}, opts...)...)
	if err != nil {
		// only reachable if a client option fails (eg an invalid APIURL override)
		clog.Log.Fatalf("creating github client: %v", err)
	}

	return client, ctx
}

// NewClient returns a github client for this repo, pointing at APIURL when set.
// It shadows the promoted Token.NewClient so all repo-scoped calls honour the override.
func (r Repo) NewClient() (*github.Client, context.Context) {
	if r.APIURL != "" {
		base := strings.TrimSuffix(r.APIURL, "/") + "/"
		return r.Token.NewClient(github.WithURLs(&base, nil))
	}

	return r.Token.NewClient()
}
