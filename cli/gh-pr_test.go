package cli

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-github/v45/github"
	"github.com/katbyte/tctest/lib/gh"
)

type MockGitHubClient struct {
	PRs              map[int]*github.PullRequest
	Files            map[int][]*github.CommitFile
	Contents         map[string]*github.RepositoryContent
	GetPRError       error
	ListFilesError   error
	GetContentsError error
}

func (m *MockGitHubClient) GetPullRequest(ctx context.Context, owner, repo string, number int) (*github.PullRequest, *github.Response, error) {
	if m.GetPRError != nil {
		return nil, nil, m.GetPRError
	}
	pr, exists := m.PRs[number]
	if !exists {
		return nil, nil, errors.New("PR not found")
	}
	return pr, &github.Response{}, nil
}

func (m *MockGitHubClient) ListPullRequestFiles(ctx context.Context, owner, repo string, number int, opts *github.ListOptions) ([]*github.CommitFile, *github.Response, error) {
	if m.ListFilesError != nil {
		return nil, nil, m.ListFilesError
	}
	files, exists := m.Files[number]
	if !exists {
		return []*github.CommitFile{}, &github.Response{}, nil
	}
	return files, &github.Response{NextPage: 0}, nil
}

func (m *MockGitHubClient) GetContents(ctx context.Context, owner, repo, path string, opts *github.RepositoryContentGetOptions) (*github.RepositoryContent, []*github.RepositoryContent, *github.Response, error) {
	if m.GetContentsError != nil {
		return nil, nil, nil, m.GetContentsError
	}
	content, exists := m.Contents[path]
	if !exists {
		return nil, nil, nil, errors.New("content not found")
	}
	return content, nil, &github.Response{}, nil
}

// MockHTTPClient implements HTTPClientInterface for testing
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

func TestPrTestsWithDependencies(t *testing.T) {
	// Setup
	gr := GithubRepo{
		Repo: gh.Repo{
			Owner: "testowner",
			Name:  "testrepo",
		},
	}

	// Mock data
	sha := "abc123"
	downloadURL := "https://example.com/file.go"

	mockGitHubClient := &MockGitHubClient{
		PRs: map[int]*github.PullRequest{
			123: {
				State:          github.String("open"),
				MergeCommitSHA: &sha,
			},
		},
		Files: map[int][]*github.CommitFile{
			123: {
				{
					Filename: github.String("internal/services/compute/test_file_test.go"),
				},
			},
		},
		Contents: map[string]*github.RepositoryContent{
			"internal/services/compute/test_file_test.go": {
				DownloadURL: &downloadURL,
			},
		},
	}

	mockHTTPClient := &MockHTTPClient{
		Responses: map[string]*http.Response{
			downloadURL: {
				StatusCode: 200,
				Body: newMockReadCloser(`package compute

func TestSomething(t *testing.T) {
	// test code
}

func TestAnotherThing(t *testing.T) {
	// more test code
}
`),
			},
		},
	}

	opts := PrTestsOptions{
		FilterRegExStr: ".*",
		SplitTestsAt:   "_",
	}

	// Test
	ctx := context.Background()
	result, err := gr.PrTestsWithDependencies(ctx, 123, opts, mockGitHubClient, mockHTTPClient)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	serviceTests := *result
	if len(serviceTests) == 0 {
		t.Fatal("Expected at least one service, got none")
	}

	// Check that we found the compute service
	computeTests, exists := serviceTests["compute"]
	if !exists {
		t.Fatal("Expected compute service in results")
	}

	if len(computeTests) != 2 {
		t.Fatalf("Expected 2 tests for compute service, got %d", len(computeTests))
	}

	// Check test names
	expectedTests := map[string]bool{
		"TestSomething":    false,
		"TestAnotherThing": false,
	}

	for _, test := range computeTests {
		if _, exists := expectedTests[test]; exists {
			expectedTests[test] = true
		}
	}

	for test, found := range expectedTests {
		if !found {
			t.Errorf("Expected to find test %s, but didn't", test)
		}
	}
}

func TestPrTestsWithDependencies_ClosedPR(t *testing.T) {
	// Setup
	gr := GithubRepo{
		Repo: gh.Repo{
			Owner: "testowner",
			Name:  "testrepo",
		},
	}

	mockGitHubClient := &MockGitHubClient{
		PRs: map[int]*github.PullRequest{
			123: {
				State: github.String("closed"),
			},
		},
	}

	mockHTTPClient := &MockHTTPClient{}

	opts := PrTestsOptions{
		FilterRegExStr: ".*",
		SplitTestsAt:   "_",
	}

	// Test
	ctx := context.Background()
	_, err := gr.PrTestsWithDependencies(ctx, 123, opts, mockGitHubClient, mockHTTPClient)

	// Assert
	if err == nil {
		t.Fatal("Expected error for closed PR, got nil")
	}

	if !strings.Contains(err.Error(), "cannot start build for a closed pr") {
		t.Errorf("Expected error message about closed PR, got: %v", err)
	}
}

func TestPrTestsWithDependencies_GitHubError(t *testing.T) {
	// Setup
	gr := GithubRepo{
		Repo: gh.Repo{
			Owner: "testowner",
			Name:  "testrepo",
		},
	}

	mockGitHubClient := &MockGitHubClient{
		GetPRError: errors.New("GitHub API error"),
	}

	mockHTTPClient := &MockHTTPClient{}

	opts := PrTestsOptions{
		FilterRegExStr: ".*",
		SplitTestsAt:   "_",
	}

	// Test
	ctx := context.Background()
	_, err := gr.PrTestsWithDependencies(ctx, 123, opts, mockGitHubClient, mockHTTPClient)

	// Assert
	if err == nil {
		t.Fatal("Expected GitHub API error, got nil")
	}

	if !strings.Contains(err.Error(), "GitHub API error") {
		t.Errorf("Expected GitHub API error message, got: %v", err)
	}
}
