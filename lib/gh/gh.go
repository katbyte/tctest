package gh

import (
	"context"
	"net/http"

	"github.com/google/go-github/v45/github"
	common "github.com/katbyte/tctest/lib/chttp"
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

// GitHubClientInterface defines the subset of GitHub client methods we need
type GitHubClientInterface interface {
	GetPullRequest(ctx context.Context, owner, repo string, number int) (*github.PullRequest, *github.Response, error)
	ListPullRequestFiles(ctx context.Context, owner, repo string, number int, opts *github.ListOptions) ([]*github.CommitFile, *github.Response, error)
	GetContents(ctx context.Context, owner, repo, path string, opts *github.RepositoryContentGetOptions) (*github.RepositoryContent, []*github.RepositoryContent, *github.Response, error)
	GetCommit(ctx context.Context, owner, repo, sha string) (*github.RepositoryCommit, *github.Response, error)
}

// HTTPClientInterface defines the subset of HTTP client methods we need
type HTTPClientInterface interface {
	Get(url string) (*http.Response, error)
}

// GitHubClientAdapter adapts the real GitHub client to our interface
type GitHubClientAdapter struct {
	client *github.Client
}

func (g *GitHubClientAdapter) GetPullRequest(ctx context.Context, owner, repo string, number int) (*github.PullRequest, *github.Response, error) {
	return g.client.PullRequests.Get(ctx, owner, repo, number)
}

func (g *GitHubClientAdapter) ListPullRequestFiles(ctx context.Context, owner, repo string, number int, opts *github.ListOptions) ([]*github.CommitFile, *github.Response, error) {
	return g.client.PullRequests.ListFiles(ctx, owner, repo, number, opts)
}

func (g *GitHubClientAdapter) GetContents(ctx context.Context, owner, repo, path string, opts *github.RepositoryContentGetOptions) (*github.RepositoryContent, []*github.RepositoryContent, *github.Response, error) {
	return g.client.Repositories.GetContents(ctx, owner, repo, path, opts)
}

func (g *GitHubClientAdapter) GetCommit(ctx context.Context, owner, repo, sha string) (*github.RepositoryCommit, *github.Response, error) {
	return g.client.Repositories.GetCommit(ctx, owner, repo, sha, nil)
}

// HTTPClientAdapter adapts the real HTTP client to our interface
type HTTPClientAdapter struct {
	client *http.Client
}

func (h *HTTPClientAdapter) Get(url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	return h.client.Do(req)
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

	httpClient.Transport = common.NewTransport("GitHub", httpClient.Transport)

	return github.NewClient(httpClient), ctx
}

// NewClientWithInterfaces returns the interfaces for dependency injection
func (t Token) NewClientWithInterfaces() (GitHubClientInterface, HTTPClientInterface, context.Context) {
	client, ctx := t.NewClient()
	httpClient := common.NewHTTPClient("HTTP")
	return &GitHubClientAdapter{client: client}, &HTTPClientAdapter{client: httpClient}, ctx
}
