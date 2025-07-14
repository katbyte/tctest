package cli

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/google/go-github/v45/github"
)

type MockGitHubClient struct {
	PRs               map[int]*github.PullRequest
	Files             map[int][]*github.CommitFile
	Contents          map[string]*github.RepositoryContent
	Commits           map[string]*github.RepositoryCommit
	DirectoryContents map[string][]*github.RepositoryContent
	GetPRError        error
	ListFilesError    error
	GetContentsError  error
	GetCommitError    error
}

func (m *MockGitHubClient) GetPullRequest(_ context.Context, _, _ string, number int) (*github.PullRequest, *github.Response, error) {
	if m.GetPRError != nil {
		return nil, nil, m.GetPRError
	}
	pr, exists := m.PRs[number]
	if !exists {
		return nil, nil, errors.New("PR not found")
	}

	return pr, &github.Response{}, nil
}

func (m *MockGitHubClient) ListPullRequestFiles(_ context.Context, _, _ string, number int, _ *github.ListOptions) ([]*github.CommitFile, *github.Response, error) {
	if m.ListFilesError != nil {
		return nil, nil, m.ListFilesError
	}
	files, exists := m.Files[number]
	if !exists {
		return []*github.CommitFile{}, &github.Response{}, nil
	}

	return files, &github.Response{NextPage: 0}, nil
}

func (m *MockGitHubClient) GetContents(_ context.Context, _, _, path string, opts *github.RepositoryContentGetOptions) (*github.RepositoryContent, []*github.RepositoryContent, *github.Response, error) {
	// Special handling for inferred test file lookups - return error to trigger fallback
	if path == "internal/services/network/security_test.go" && opts.Ref != "main" {
		return nil, nil, nil, errors.New("content not found")
	}

	if m.GetContentsError != nil {
		return nil, nil, nil, m.GetContentsError
	}
	if dirContents, exists := m.DirectoryContents[path]; exists {
		return nil, dirContents, &github.Response{}, nil
	}
	content, exists := m.Contents[path]
	if !exists {
		return nil, nil, nil, errors.New("content not found")
	}

	return content, nil, &github.Response{}, nil
}

func (m *MockGitHubClient) GetCommit(_ context.Context, _, _ string, sha string) (*github.RepositoryCommit, *github.Response, error) {
	if m.GetCommitError != nil {
		return nil, nil, m.GetCommitError
	}
	commit, exists := m.Commits[sha]
	if !exists {
		return nil, nil, errors.New("commit not found")
	}

	return commit, &github.Response{}, nil
}

type MockHTTPClient struct {
	Responses map[string]*http.Response
	GetError  error
}

func (m *MockHTTPClient) Get(url string) (*http.Response, error) {
	if m.GetError != nil {
		return nil, m.GetError
	}
	resp, exists := m.Responses[url]
	if !exists {
		return nil, errors.New("URL not found")
	}

	return resp, nil
}

// mockReadCloser creates a mock ReadCloser for testing
type mockReadCloser struct {
	*strings.Reader
}

func (m mockReadCloser) Close() error {
	return nil
}

func newMockReadCloser(content string) io.ReadCloser {
	return mockReadCloser{strings.NewReader(content)}
}
