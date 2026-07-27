package gh

import (
	"context"
	"net/url"
	"strings"

	"github.com/google/go-github/v45/github"
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

	if t := t.Token; t != nil {
		t := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: *t},
		)
		httpClient = oauth2.NewClient(ctx, t)
	}

	httpClient.Transport = common.NewRetryTransport("GitHub", common.NewTransport("GitHub", httpClient.Transport), 3)

	return github.NewClient(httpClient), ctx
}

// NewClient returns a github client for this repo, pointing at APIURL when set.
// It shadows the promoted Token.NewClient so all repo-scoped calls honour the override.
func (r Repo) NewClient() (*github.Client, context.Context) {
	client, ctx := r.Token.NewClient()

	if r.APIURL != "" {
		if u, err := url.Parse(strings.TrimSuffix(r.APIURL, "/") + "/"); err == nil {
			client.BaseURL = u
		} else {
			clog.Log.Errorf("invalid github api url %q, using default: %v", r.APIURL, err)
		}
	}

	return client, ctx
}
