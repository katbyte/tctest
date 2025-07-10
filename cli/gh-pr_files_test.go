package cli

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/go-github/v45/github"
	"github.com/katbyte/tctest/lib/gh"
)

func TestParseTestsFromFiles(t *testing.T) {
	gr := GithubRepo{
		Repo: gh.Repo{
			Owner: "testowner",
			Name:  "testrepo",
		},
	}

	downloadURL1 := "https://example.com/compute_test.go"
	downloadURL2 := "https://example.com/storage_test.go"

	mockGitHubClient := &MockGitHubClient{
		Contents: map[string]*github.RepositoryContent{
			"internal/services/compute/compute_test.go": {
				DownloadURL: &downloadURL1,
			},
			"internal/services/storage/storage_test.go": {
				DownloadURL: &downloadURL2,
			},
		},
	}

	mockHTTPClient := &MockHTTPClient{
		Responses: map[string]*http.Response{
			downloadURL1: {
				StatusCode: 200,
				Body: newMockReadCloser(`package compute

func TestResource(t *testing.T) {
	// test code
}

func TestDataSource(t *testing.T) {
	// test code
}

func TestValidation_Helper(t *testing.T) {
	// helper test
}
`),
			},
			downloadURL2: {
				StatusCode: 200,
				Body: newMockReadCloser(`package storage

func TestAccount(t *testing.T) {
	// test code
}

func TestBlob_Complex(t *testing.T) {
	// test code
}
`),
			},
		},
	}

	filesFiltered := map[string]struct{}{
		"internal/services/compute/compute_test.go": {},
		"internal/services/storage/storage_test.go": {},
	}

	pr := &github.PullRequest{
		MergeCommitSHA: github.String("sha123"),
	}

	ctx := context.Background()
	result, err := gr.parseTestsFromFiles(ctx, filesFiltered, pr, "_", mockGitHubClient, mockHTTPClient)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	serviceTests := *result
	if len(serviceTests) != 2 {
		t.Fatalf("Expected 2 services, got %d", len(serviceTests))
	}

	// Check compute service tests
	computeTests, exists := serviceTests["compute"]
	if !exists {
		t.Fatal("Expected compute service in results")
	}
	if len(computeTests) != 3 {
		t.Fatalf("Expected 3 tests for compute service, got %d", len(computeTests))
	}

	expectedComputeTests := map[string]bool{
		"TestResource":   false,
		"TestDataSource": false,
		"TestValidation": false, // Should be split at "_"
	}

	for _, test := range computeTests {
		if _, exists := expectedComputeTests[test]; exists {
			expectedComputeTests[test] = true
		}
	}

	for test, found := range expectedComputeTests {
		if !found {
			t.Errorf("Expected to find compute test %s, but didn't", test)
		}
	}

	// Check storage service tests
	storageTests, exists := serviceTests["storage"]
	if !exists {
		t.Fatal("Expected storage service in results")
	}
	if len(storageTests) != 2 {
		t.Fatalf("Expected 2 tests for storage service, got %d", len(storageTests))
	}

	expectedStorageTests := map[string]bool{
		"TestAccount": false,
		"TestBlob":    false, // Should be split at "_"
	}

	for _, test := range storageTests {
		if _, exists := expectedStorageTests[test]; exists {
			expectedStorageTests[test] = true
		}
	}

	for test, found := range expectedStorageTests {
		if !found {
			t.Errorf("Expected to find storage test %s, but didn't", test)
		}
	}
}

func TestParseTestsFromFiles_NilMergeCommitSHA(t *testing.T) {
	gr := GithubRepo{
		Repo: gh.Repo{
			Owner: "testowner",
			Name:  "testrepo",
		},
	}

	filesFiltered := map[string]struct{}{
		"test_file.go": {},
	}

	pr := &github.PullRequest{
		MergeCommitSHA: nil,
	}

	ctx := context.Background()
	_, err := gr.parseTestsFromFiles(ctx, filesFiltered, pr, "_", &MockGitHubClient{}, &MockHTTPClient{})

	if err == nil {
		t.Fatal("Expected error for nil MergeCommitSHA, got nil")
	}

	if err.Error() != "merge commit SHA is nil, is there a merge conflict?" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestParseTestsFromFiles_NoContents(t *testing.T) {
	gr := GithubRepo{
		Repo: gh.Repo{
			Owner: "testowner",
			Name:  "testrepo",
		},
	}

	mockGitHubClient := &MockGitHubClient{
		GetContentsError: errors.New("content not found"),
	}

	filesFiltered := map[string]struct{}{
		"test_file.go": {},
	}

	pr := &github.PullRequest{
		MergeCommitSHA: github.String("sha123"),
	}

	ctx := context.Background()
	_, err := gr.parseTestsFromFiles(ctx, filesFiltered, pr, "_", mockGitHubClient, &MockHTTPClient{})

	if err == nil {
		t.Fatal("Expected error for no contents, got nil")
	}

	if err.Error() != "downloading file (test_file.go): no contents" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestParseTestsFromFiles_MissingDownloadURL(t *testing.T) {
	gr := GithubRepo{
		Repo: gh.Repo{
			Owner: "testowner",
			Name:  "testrepo",
		},
	}

	mockGitHubClient := &MockGitHubClient{
		Contents: map[string]*github.RepositoryContent{
			"test_file.go": {
				DownloadURL: nil,
			},
		},
	}

	filesFiltered := map[string]struct{}{
		"test_file.go": {},
	}

	pr := &github.PullRequest{
		MergeCommitSHA: github.String("sha123"),
	}

	ctx := context.Background()
	_, err := gr.parseTestsFromFiles(ctx, filesFiltered, pr, "_", mockGitHubClient, &MockHTTPClient{})

	if err == nil {
		t.Fatal("Expected error for missing DownloadURL, got nil")
	}

	if err.Error() != "downloading file (test_file.go): missing DownloadURL" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestGetAllPullRequestFilesWithClient(t *testing.T) {
	gr := GithubRepo{
		Repo: gh.Repo{
			Owner: "testowner",
			Name:  "testrepo",
		},
	}

	downloadURL := "https://example.com/inferred_test.go"

	mockGitHubClient := &MockGitHubClient{
		PRs: map[int]*github.PullRequest{
			123: {
				State:          github.String("open"),
				MergeCommitSHA: github.String("sha123"),
			},
		},
		Files: map[int][]*github.CommitFile{
			123: {
				{
					Filename: github.String("internal/services/compute/resource.go"),
				},
				{
					Filename: github.String("internal/services/compute/resource_test.go"),
				},
				{
					Filename: github.String("internal/services/storage/data_source.go"),
				},
				{
					Filename: github.String("internal/services/network/client/client.go"), // Should be skipped
				},
				{
					Filename: github.String("internal/services/network/registration.go"), // Should be skipped
				},
			},
		},
		Contents: map[string]*github.RepositoryContent{
			"internal/services/compute/resource_test.go": {
				DownloadURL: &downloadURL,
			},
			"internal/services/storage/data_source_test.go": {
				DownloadURL: &downloadURL,
			},
		},
	}

	ctx := context.Background()
	result, err := gr.getAllPullRequestFilesWithClient(ctx, 123, ".*", mockGitHubClient)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	files := *result
	expectedFiles := map[string]bool{
		"internal/services/compute/resource_test.go":    false, // Inferred test file (same as direct)
		"internal/services/storage/data_source_test.go": false, // Inferred test file
	}

	// The result should contain:
	// 1. Direct test files from PR
	// 2. Inferred test files for production code files
	if len(files) < 2 {
		t.Fatalf("Expected at least 2 files, got %d: %v", len(files), files)
	}

	// Check for presence of expected files
	for file := range files {
		if _, exists := expectedFiles[file]; exists {
			expectedFiles[file] = true
		}
	}

	// Verify we found the direct test file
	if _, exists := files["internal/services/compute/resource_test.go"]; !exists {
		t.Error("Expected to find direct test file 'internal/services/compute/resource_test.go'")
	}

	// Verify we found the inferred test file
	if _, exists := files["internal/services/storage/data_source_test.go"]; !exists {
		t.Error("Expected to find inferred test file 'internal/services/storage/data_source_test.go'")
	}
}

func TestGetAllPullRequestFilesWithClient_NilMergeCommitSHA(t *testing.T) {
	gr := GithubRepo{
		Repo: gh.Repo{
			Owner: "testowner",
			Name:  "testrepo",
		},
	}

	mockGitHubClient := &MockGitHubClient{
		PRs: map[int]*github.PullRequest{
			123: {
				State:          github.String("open"),
				MergeCommitSHA: nil,
			},
		},
	}

	ctx := context.Background()
	_, err := gr.getAllPullRequestFilesWithClient(ctx, 123, ".*", mockGitHubClient)

	if err == nil {
		t.Fatal("Expected error for nil MergeCommitSHA, got nil")
	}

	if err.Error() != "merge commit SHA is nil" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestGetAllPullRequestFilesWithClient_PRNotFound(t *testing.T) {
	gr := GithubRepo{
		Repo: gh.Repo{
			Owner: "testowner",
			Name:  "testrepo",
		},
	}

	mockGitHubClient := &MockGitHubClient{
		GetPRError: errors.New("PR not found"),
	}

	ctx := context.Background()
	_, err := gr.getAllPullRequestFilesWithClient(ctx, 123, ".*", mockGitHubClient)

	if err == nil {
		t.Fatal("Expected error for PR not found, got nil")
	}

	if !contains(err.Error(), "failed to get PR") {
		t.Errorf("Expected error to contain 'failed to get PR', got: %v", err)
	}
}

func TestGetAllPullRequestFilesWithClient_ListFilesError(t *testing.T) {
	gr := GithubRepo{
		Repo: gh.Repo{
			Owner: "testowner",
			Name:  "testrepo",
		},
	}

	mockGitHubClient := &MockGitHubClient{
		PRs: map[int]*github.PullRequest{
			123: {
				State:          github.String("open"),
				MergeCommitSHA: github.String("sha123"),
			},
		},
		ListFilesError: errors.New("GitHub API error"),
	}

	ctx := context.Background()
	_, err := gr.getAllPullRequestFilesWithClient(ctx, 123, ".*", mockGitHubClient)

	if err == nil {
		t.Fatal("Expected error for list files error, got nil")
	}

	if !contains(err.Error(), "failed to get all files") {
		t.Errorf("Expected error to contain 'failed to get all files', got: %v", err)
	}
}

func TestGetAllPullRequestFilesWithClient_FallbackToPackageTests(t *testing.T) {
	gr := GithubRepo{
		Repo: gh.Repo{
			Owner: "testowner",
			Name:  "testrepo",
		},
	}

	mockGitHubClient := &MockGitHubClient{
		PRs: map[int]*github.PullRequest{
			123: {
				State:          github.String("open"),
				MergeCommitSHA: github.String("sha123"),
			},
		},
		Files: map[int][]*github.CommitFile{
			123: {
				{
					Filename: github.String("internal/services/network/security.go"),
				},
			},
		},
		DirectoryContents: map[string][]*github.RepositoryContent{
			"internal/services/network": {
				{Name: github.String("security_test.go"), Type: github.String("file")},
				{Name: github.String("other_test.go"), Type: github.String("file")},
				{Name: github.String("helper.go"), Type: github.String("file")}, // Should be ignored (not _test.go)
			},
		},
	}

	ctx := context.Background()
	result, err := gr.getAllPullRequestFilesWithClient(ctx, 123, ".*", mockGitHubClient)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	files := *result

	// Should contain fallback test files from the same package
	expectedFiles := []string{
		"internal/services/network/security_test.go",
		"internal/services/network/other_test.go",
	}

	for _, expectedFile := range expectedFiles {
		if _, exists := files[expectedFile]; !exists {
			t.Errorf("Expected to find fallback test file '%s'", expectedFile)
		}
	}

	// Should not contain the helper.go file
	if _, exists := files["internal/services/network/helper.go"]; exists {
		t.Error("Should not include non-test files in fallback")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
