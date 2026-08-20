package integration

import (
	"strings"
	"testing"
)

var awsPRs = []prDef{
	{10001, "open", "changed resource file", []changedFile{
		{"internal/service/rekognition/collection.go", "modified"},
	}},
	{10002, "open", "two resources one service", []changedFile{
		{"internal/service/rekognition/collection.go", "modified"},
		{"internal/service/rekognition/project.go", "modified"},
	}},
	{10003, "open", "changed sdk resource without _resource suffix", []changedFile{
		{"internal/service/s3/bucket.go", "modified"},
	}},
	{10004, "open", "changed test file", []changedFile{
		{"internal/service/s3/bucket_test.go", "modified"},
	}},
	{10005, "open", "multiple services", []changedFile{
		{"internal/service/rekognition/collection.go", "modified"},
		{"internal/service/s3/bucket.go", "modified"},
	}},
	{10006, "open", "generated and export files only", []changedFile{
		{"internal/service/rekognition/service_package_gen.go", "modified"},
		{"internal/service/rekognition/exports_test.go", "modified"},
		{"internal/service/rekognition/tags_gen.go", "modified"},
	}},
	{10007, "open", "changed plural data source", []changedFile{
		{"internal/service/s3/buckets_data_source.go", "modified"},
	}},
	{10008, "open", "changed flat migrate helper", []changedFile{
		{"internal/service/rekognition/stream_processor_migrate.go", "modified"},
	}},
}

// awsASTPRs are served for the aws AST-mode tests; the merge refs for these
// numbers exist in the aws git fixture upstream.
var awsASTPRs = []prDef{
	{20001, "open", "changed sdk resource without _resource suffix", []changedFile{
		{"internal/service/s3/bucket.go", "modified"},
	}},
	{20002, "open", "changed framework resource", []changedFile{
		{"internal/service/rekognition/stream_processor.go", "modified"},
	}},
	{20003, "open", "multiple services", []changedFile{
		{"internal/service/rekognition/collection.go", "modified"},
		{"internal/service/s3/bucket.go", "modified"},
	}},
	{20004, "open", "generated and export files only", []changedFile{
		{"internal/service/rekognition/service_package_gen.go", "modified"},
		{"internal/service/rekognition/exports_test.go", "modified"},
		{"internal/service/rekognition/tags_gen.go", "modified"},
	}},
}

func awsEnv(gh *mockGitHub, tc *mockTeamCity) map[string]string {
	return map[string]string{
		"TCTEST_SERVER":         tc.srv.URL,
		"TCTEST_REPO":           "hashicorp/terraform-provider-aws",
		"TCTEST_GITHUB_API_URL": gh.apiURL(),
		"TCTEST_GITHUB_RAW_URL": gh.rawURL(),
		// aws uses a single build configuration; the service is not part of the build type id
	}
}

// TestAPIDiscoveryAWS covers the aws-shaped fixture: singular internal/service/
// paths, generated-test suffixes, and a single build configuration (no suffix).
func TestAPIDiscoveryAWS(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		args     []string
		want     []trigger
		wantExit int
	}{
		{
			name: "resource change derives plain and generated test files",
			args: []string{"pr", "10001"},
			want: []trigger{{"TF_E2E", "refs/pull/10001/merge", "(TestAccRekognitionCollection)"}},
		},
		{
			name: "two resources in one service produce one combined build",
			args: []string{"pr", "10002"},
			want: []trigger{{"TF_E2E", "refs/pull/10002/merge", "(TestAccRekognitionCollection|TestAccRekognitionProject)"}},
		},
		{
			// bucket.go has no _resource suffix; the fileregex promotes it, and its
			// prefix must pull in bucket/identity/data-source tests but NOT the
			// plural buckets_data_source tests
			name: "sdk resource without _resource suffix derives its test family",
			args: []string{"pr", "10003"},
			want: []trigger{{"TF_E2E", "refs/pull/10003/merge", "(TestAccS3Bucket|TestAccS3BucketDataSource)"}},
		},
		{
			name: "changed test file runs its tests",
			args: []string{"pr", "10004"},
			want: []trigger{{"TF_E2E", "refs/pull/10004/merge", "(TestAccS3Bucket)"}},
		},
		{
			// without a build-type suffix both services trigger the same build
			// configuration, differing only in TEST_PATTERN
			name: "multi-service pr triggers one build per service",
			args: []string{"pr", "10005"},
			want: []trigger{
				{"TF_E2E", "refs/pull/10005/merge", "(TestAccRekognitionCollection)"},
				{"TF_E2E", "refs/pull/10005/merge", "(TestAccS3Bucket|TestAccS3BucketDataSource)"},
			},
		},
		{
			name:     "generated and export files only trigger nothing and fail",
			args:     []string{"pr", "10006"},
			want:     nil,
			wantExit: 1,
		},
		{
			name: "plural data source derives only its own test",
			args: []string{"pr", "10007"},
			want: []trigger{{"TF_E2E", "refs/pull/10007/merge", "(TestAccS3BucketsDataSource)"}},
		},
		{
			// documents a current limitation: AWS keeps migrate helpers flat in the
			// service dir (stream_processor_migrate.go), the prefix match finds no
			// sibling tests, so nothing runs even though stream_processor tests
			// arguably should
			name:     "flat migrate helper alone triggers nothing and fails",
			args:     []string{"pr", "10008"},
			want:     nil,
			wantExit: 1,
		},
		{
			name: "--service filters a multi-service pr",
			args: []string{"pr", "10005", "--service", "s3"},
			want: []trigger{{"TF_E2E", "refs/pull/10005/merge", "(TestAccS3Bucket|TestAccS3BucketDataSource)"}},
		},
		{
			// ListServices falls through internal/services (10004) to the aws
			// singular internal/service layout
			name: "--service all triggers every service directly",
			args: []string{"pr", "10001", "--service", "all", "--all"},
			want: []trigger{
				{"TF_E2E", "refs/pull/10001/merge", "TestAcc"},
				{"TF_E2E", "refs/pull/10001/merge", "TestAcc"},
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			scenario(t, "api/aws", tt.name)
			gh := newMockGitHub(t, "testdata/aws", awsPRs)
			tc := newMockTeamCity(t)

			res := runTCTest(t, awsEnv(gh, tc), tt.args...)
			if res.exitCode != tt.wantExit {
				t.Fatalf("exit code = %d, want %d\noutput:\n%s", res.exitCode, tt.wantExit, res.output)
			}
			assertTriggers(t, tc, res, tt.want)
		})
	}
}

// TestASTDiscoveryAWS mirrors the aws API-mode cases through the local AST
// path: singular internal/service/ layout, framework and SDK-classic resources,
// generated files, and content-based unit-test classification (exports_test.go).
func TestASTDiscoveryAWS(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		args     []string
		want     []trigger
		wantExit int
	}{
		{
			name: "sdk resource without _resource suffix derives its test family",
			args: []string{"pr", "20001"},
			want: []trigger{{"TF_E2E", "refs/pull/20001/merge", "(TestAccS3Bucket|TestAccS3BucketDataSource)"}},
		},
		{
			name: "framework resource derives its tests",
			args: []string{"pr", "20002"},
			want: []trigger{{"TF_E2E", "refs/pull/20002/merge", "(TestAccRekognitionStreamProcessor)"}},
		},
		{
			name: "multi-service pr triggers one build per service",
			args: []string{"pr", "20003"},
			want: []trigger{
				{"TF_E2E", "refs/pull/20003/merge", "(TestAccRekognitionCollection)"},
				{"TF_E2E", "refs/pull/20003/merge", "(TestAccS3Bucket|TestAccS3BucketDataSource)"},
			},
		},
		{
			// exports_test.go is classified as a unit test here because AST mode
			// reads its content (no TestAcc funcs), unlike the API path's
			// filename-based fallback
			name:     "generated and export files only trigger nothing and fail",
			args:     []string{"pr", "20004"},
			want:     nil,
			wantExit: 1,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			scenario(t, "ast/aws", tt.name)
			gh := newMockGitHub(t, "testdata/aws", awsASTPRs)
			tc := newMockTeamCity(t)
			clone := cloneUpstream(t, awsUpstream)

			env := awsEnv(gh, tc)
			env["TCTEST_LOCAL_REPO_PATH"] = clone

			res := runTCTest(t, env, tt.args...)
			if res.exitCode != tt.wantExit {
				t.Fatalf("exit code = %d, want %d\noutput:\n%s", res.exitCode, tt.wantExit, res.output)
			}
			if !strings.Contains(res.output, "[mode=AST]") {
				t.Fatalf("expected AST discovery mode to be used\noutput:\n%s", res.output)
			}
			assertTriggers(t, tc, res, tt.want)
		})
	}
}
