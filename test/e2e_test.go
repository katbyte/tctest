package test

import (
	"encoding/json"
	"maps"
	"regexp"
	"strings"
	"testing"
)

// azurermPRs defines the PRs the mock GitHub serves for the azurerm fixture.
var azurermPRs = []prDef{
	{201, "open", "changed test file", []changedFile{
		{"internal/services/postgres/postgresql_flexible_server_resource_test.go", "modified"},
	}},
	{202, "open", "changed resource file", []changedFile{
		{"internal/services/postgres/postgresql_flexible_server_resource.go", "modified"},
	}},
	{203, "open", "changed cross-package helper only", []changedFile{
		{"internal/services/postgres/validate/database_charset.go", "modified"},
	}},
	{204, "open", "multiple services", []changedFile{
		{"internal/services/postgres/postgresql_flexible_server_resource_test.go", "modified"},
		{"internal/services/dns/dns_a_record_resource.go", "modified"},
	}},
	{205, "closed", "closed pr", []changedFile{
		{"internal/services/postgres/postgresql_flexible_server_resource_test.go", "modified"},
	}},
	{206, "open", "changed untyped data source", []changedFile{
		{"internal/services/postgres/postgresql_flexible_server_data_source.go", "modified"},
	}},
	{207, "open", "changed typed data source", []changedFile{
		{"internal/services/dns/dns_zone_data_source.go", "modified"},
	}},
	{208, "open", "mixed resource, test, and helper", []changedFile{
		{"internal/services/postgres/postgresql_flexible_server_resource.go", "modified"},
		{"internal/services/postgres/postgresql_flexible_server_resource_test.go", "modified"},
		{"internal/services/postgres/validate/database_charset.go", "modified"},
	}},
	{220, "open", "postgres improvement", []changedFile{
		{"internal/services/postgres/postgresql_flexible_server_resource_test.go", "modified"},
	}},
	{221, "open", "dns bug fix", []changedFile{
		{"internal/services/dns/dns_a_record_resource.go", "modified"},
	}},
}

// azurermASTPRs are served for the AST-mode tests; the merge refs for these
// numbers exist in the git fixture upstream.
var azurermASTPRs = []prDef{
	{301, "open", "same-package helper changed", []changedFile{
		{"internal/services/dns/ipv6_address.go", "modified"},
	}},
	{302, "open", "cross-package helper changed", []changedFile{
		{"internal/services/postgres/validate/database_charset.go", "modified"},
	}},
	{303, "open", "two-level helper chain changed", []changedFile{
		{"internal/services/postgres/parse/postgresql_aad_administrator.go", "modified"},
	}},
	{304, "open", "vendored dependency changed", []changedFile{
		{"vendor/github.com/hashicorp/go-azure-sdk/resource-manager/cosmosdb/2024-08-15/cosmosdb/client.go", "modified"},
	}},
	{305, "open", "changed test file", []changedFile{
		{"internal/services/postgres/postgresql_flexible_server_resource_test.go", "modified"},
	}},
	{306, "open", "changed resource file", []changedFile{
		{"internal/services/postgres/postgresql_flexible_server_resource.go", "modified"},
	}},
	{307, "open", "multiple services", []changedFile{
		{"internal/services/postgres/postgresql_flexible_server_resource_test.go", "modified"},
		{"internal/services/dns/dns_a_record_resource.go", "modified"},
	}},
	{308, "closed", "closed pr", []changedFile{
		{"internal/services/postgres/postgresql_flexible_server_resource_test.go", "modified"},
	}},
	{309, "open", "changed untyped data source", []changedFile{
		{"internal/services/postgres/postgresql_flexible_server_data_source.go", "modified"},
	}},
	{310, "open", "changed typed data source", []changedFile{
		{"internal/services/dns/dns_zone_data_source.go", "modified"},
	}},
	{311, "open", "mixed resource, test, and helper", []changedFile{
		{"internal/services/postgres/postgresql_flexible_server_resource.go", "modified"},
		{"internal/services/postgres/postgresql_flexible_server_resource_test.go", "modified"},
		{"internal/services/postgres/validate/database_charset.go", "modified"},
	}},
}

var awsPRs = []prDef{
	{401, "open", "changed resource file", []changedFile{
		{"internal/service/rekognition/collection.go", "modified"},
	}},
	{402, "open", "two resources one service", []changedFile{
		{"internal/service/rekognition/collection.go", "modified"},
		{"internal/service/rekognition/project.go", "modified"},
	}},
	{403, "open", "changed sdk resource without _resource suffix", []changedFile{
		{"internal/service/s3/bucket.go", "modified"},
	}},
	{404, "open", "changed test file", []changedFile{
		{"internal/service/s3/bucket_test.go", "modified"},
	}},
	{405, "open", "multiple services", []changedFile{
		{"internal/service/rekognition/collection.go", "modified"},
		{"internal/service/s3/bucket.go", "modified"},
	}},
	{406, "open", "generated and export files only", []changedFile{
		{"internal/service/rekognition/service_package_gen.go", "modified"},
		{"internal/service/rekognition/exports_test.go", "modified"},
		{"internal/service/rekognition/tags_gen.go", "modified"},
	}},
	{407, "open", "changed plural data source", []changedFile{
		{"internal/service/s3/buckets_data_source.go", "modified"},
	}},
	{408, "open", "changed flat migrate helper", []changedFile{
		{"internal/service/rekognition/stream_processor_migrate.go", "modified"},
	}},
}

func azurermEnv(gh *mockGitHub, tc *mockTeamCity) map[string]string {
	return map[string]string{
		"TCTEST_SERVER":         tc.srv.URL, // explicit http:// scheme points at the mock
		"TCTEST_REPO":           "hashicorp/terraform-provider-azurerm",
		"TCTEST_GITHUB_API_URL": gh.apiURL(),
		"TCTEST_GITHUB_RAW_URL": gh.rawURL(),
		// azurerm-style per-service build configurations
		"TCTEST_BUILD_TYPE_ID_ADD_SERVICE_SUFFIX": "true",
	}
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

// TestAPIDiscoveryAzureRM covers the HTTP-API discovery path against the
// azurerm-shaped fixture (plural internal/services/, per-service build configs).
func TestAPIDiscoveryAzureRM(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		args     []string
		want     []trigger
		wantExit int
	}{
		{
			name: "changed test file runs its tests",
			args: []string{"pr", "201"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/201/merge", "(TestAccPostgresqlFlexibleServer)"}},
		},
		{
			name: "changed resource file runs derived sibling tests",
			args: []string{"pr", "202"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/202/merge", "(TestAccDataSourcePostgresqlflexibleServer|TestAccPostgresqlFlexibleServer)"}},
		},
		{
			name: "helper-only change discovers nothing in API mode",
			args: []string{"pr", "203"},
			want: nil,
		},
		{
			name: "multi-service pr triggers one build per service",
			args: []string{"pr", "204"},
			want: []trigger{
				{"TF_E2E_DNS", "refs/pull/204/merge", "(TestAccDnsARecord)"},
				{"TF_E2E_POSTGRES", "refs/pull/204/merge", "(TestAccPostgresqlFlexibleServer)"},
			},
		},
		{
			name:     "closed pr errors and triggers nothing",
			args:     []string{"pr", "205"},
			want:     nil,
			wantExit: 1,
		},
		{
			name: "explicit test regex overrides discovery",
			args: []string{"pr", "201", "TestAccPostgresqlFlexibleServer_complete"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/201/merge", "TestAccPostgresqlFlexibleServer_complete"}},
		},
		{
			name: "--all overrides the discovered regex with TestAcc",
			args: []string{"pr", "201", "--all"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/201/merge", "TestAcc"}},
		},
		{
			name: "--service filters a multi-service pr",
			args: []string{"pr", "204", "--service", "postgres"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/204/merge", "(TestAccPostgresqlFlexibleServer)"}},
		},
		{
			name: "changed untyped data source runs its derived test",
			args: []string{"pr", "206"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/206/merge", "(TestAccDataSourcePostgresqlflexibleServer)"}},
		},
		{
			name: "changed typed data source runs its derived test",
			args: []string{"pr", "207"},
			want: []trigger{{"TF_E2E_DNS", "refs/pull/207/merge", "(TestAccAzureRMDNSZoneDataSource)"}},
		},
		{
			// helpers don't trace in API mode, so only the resource's tests run;
			// the AST-mode mirror of this PR also picks up the traced database tests
			name: "mixed pr runs changed and derived tests only",
			args: []string{"pr", "208"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/208/merge", "(TestAccDataSourcePostgresqlflexibleServer|TestAccPostgresqlFlexibleServer)"}},
		},
		{
			name: "--add-tests appends to the discovered regex",
			args: []string{"pr", "202", "--add-tests", "TestAccExtraThing"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/202/merge", "(TestAccDataSourcePostgresqlflexibleServer|TestAccPostgresqlFlexibleServer|TestAccExtraThing)"}},
		},
		{
			name:     "--max-builds-per-pr limit errors and triggers nothing",
			args:     []string{"pr", "204", "--max-builds-per-pr", "1"},
			want:     nil,
			wantExit: 1,
		},
		{
			name: "--dry-run triggers nothing and exits cleanly",
			args: []string{"pr", "201", "--dry-run"},
			want: nil,
		},
		{
			name: "--service all triggers every service directly",
			args: []string{"pr", "201", "--service", "all", "--all"},
			want: []trigger{
				{"TF_E2E_COSMOS", "refs/pull/201/merge", "TestAcc"},
				{"TF_E2E_DNS", "refs/pull/201/merge", "TestAcc"},
				{"TF_E2E_POSTGRES", "refs/pull/201/merge", "TestAcc"},
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			scenario(t, "api/azurerm", tt.name)
			gh := newMockGitHub(t, "testdata/azurerm", azurermPRs)
			tc := newMockTeamCity(t)

			res := runTCTest(t, azurermEnv(gh, tc), tt.args...)
			if res.exitCode != tt.wantExit {
				t.Fatalf("exit code = %d, want %d\noutput:\n%s", res.exitCode, tt.wantExit, res.output)
			}
			assertTriggers(t, tc, res, tt.want)
		})
	}
}

// TestAPIDiscoveryAWS covers the aws-shaped fixture: singular internal/service/
// paths, generated-test suffixes, and a single build configuration (no suffix).
func TestAPIDiscoveryAWS(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want []trigger
	}{
		{
			name: "resource change derives plain and generated test files",
			args: []string{"pr", "401"},
			want: []trigger{{"TF_E2E", "refs/pull/401/merge", "(TestAccRekognitionCollection)"}},
		},
		{
			name: "two resources in one service produce one combined build",
			args: []string{"pr", "402"},
			want: []trigger{{"TF_E2E", "refs/pull/402/merge", "(TestAccRekognitionCollection|TestAccRekognitionProject)"}},
		},
		{
			// bucket.go has no _resource suffix; the fileregex promotes it, and its
			// prefix must pull in bucket/identity/data-source tests but NOT the
			// plural buckets_data_source tests
			name: "sdk resource without _resource suffix derives its test family",
			args: []string{"pr", "403"},
			want: []trigger{{"TF_E2E", "refs/pull/403/merge", "(TestAccS3Bucket|TestAccS3BucketDataSource)"}},
		},
		{
			name: "changed test file runs its tests",
			args: []string{"pr", "404"},
			want: []trigger{{"TF_E2E", "refs/pull/404/merge", "(TestAccS3Bucket)"}},
		},
		{
			// without a build-type suffix both services trigger the same build
			// configuration, differing only in TEST_PATTERN
			name: "multi-service pr triggers one build per service",
			args: []string{"pr", "405"},
			want: []trigger{
				{"TF_E2E", "refs/pull/405/merge", "(TestAccRekognitionCollection)"},
				{"TF_E2E", "refs/pull/405/merge", "(TestAccS3Bucket|TestAccS3BucketDataSource)"},
			},
		},
		{
			name: "generated and export files only trigger nothing",
			args: []string{"pr", "406"},
			want: nil,
		},
		{
			name: "plural data source derives only its own test",
			args: []string{"pr", "407"},
			want: []trigger{{"TF_E2E", "refs/pull/407/merge", "(TestAccS3BucketsDataSource)"}},
		},
		{
			// documents a current limitation: AWS keeps migrate helpers flat in the
			// service dir (stream_processor_migrate.go), the prefix match finds no
			// sibling tests, so nothing runs even though stream_processor tests
			// arguably should
			name: "flat migrate helper alone triggers nothing",
			args: []string{"pr", "408"},
			want: nil,
		},
		{
			name: "--service filters a multi-service pr",
			args: []string{"pr", "405", "--service", "s3"},
			want: []trigger{{"TF_E2E", "refs/pull/405/merge", "(TestAccS3Bucket|TestAccS3BucketDataSource)"}},
		},
		{
			// ListServices falls through internal/services (404) to the aws
			// singular internal/service layout
			name: "--service all triggers every service directly",
			args: []string{"pr", "401", "--service", "all", "--all"},
			want: []trigger{
				{"TF_E2E", "refs/pull/401/merge", "TestAcc"},
				{"TF_E2E", "refs/pull/401/merge", "TestAcc"},
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
			if res.exitCode != 0 {
				t.Fatalf("exit code = %d, want 0\noutput:\n%s", res.exitCode, res.output)
			}
			assertTriggers(t, tc, res, tt.want)
		})
	}
}

// TestASTDiscoveryAzureRM covers the local AST-based discovery path: the binary
// fetches and checks out real PR merge refs from a local git upstream, then
// traces helper and vendor changes to affected resources.
func TestASTDiscoveryAzureRM(t *testing.T) {
	t.Parallel()

	// matches only resource/data-source files directly in a service dir, so the
	// shared dns ipv6_address.go helper stays a helper (the default regex would claim it)
	const narrowFileRegex = `^internal/services/[a-z]+/[a-z0-9_]+(resource|data_source)\.go$`

	cases := []struct {
		name     string
		args     []string
		extra    map[string]string
		want     []trigger
		wantExit int
	}{
		{
			name:  "same-package helper traces to resources using its symbols",
			args:  []string{"pr", "301"},
			extra: map[string]string{"TCTEST_FILEREGEX": narrowFileRegex},
			want:  []trigger{{"TF_E2E_DNS", "refs/pull/301/merge", "(TestAccDataSourceDnsAAAARecord|TestAccDnsAAAARecord)"}},
		},
		{
			name: "cross-package helper traces through import symbol usage",
			args: []string{"pr", "302"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/302/merge", "(TestAccPostgresqlFlexibleServerDatabase)"}},
		},
		{
			name: "two-level helper chain traces only symbol users",
			args: []string{"pr", "303"},
			// the virtual endpoint resource imports the same migration package but
			// uses an upgrader from a different file — it must NOT be selected
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/303/merge", "(TestAccPostgreSQLAdministrator)"}},
		},
		{
			name: "vendored dependency change traces to importing resources",
			args: []string{"pr", "304"},
			want: []trigger{{"TF_E2E_COSMOS", "refs/pull/304/merge", "(TestAccCosmosDBAccount|TestAccDataSourceCosmosDBAccount)"}},
		},

		// the simple discovery cases covered by the API-mode tests, mirrored
		// here to prove the local AST path handles them identically
		{
			name: "changed test file runs its tests",
			args: []string{"pr", "305"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/305/merge", "(TestAccPostgresqlFlexibleServer)"}},
		},
		{
			name: "changed resource file runs derived sibling tests",
			args: []string{"pr", "306"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/306/merge", "(TestAccDataSourcePostgresqlflexibleServer|TestAccPostgresqlFlexibleServer)"}},
		},
		{
			name: "multi-service pr triggers one build per service",
			args: []string{"pr", "307"},
			want: []trigger{
				{"TF_E2E_DNS", "refs/pull/307/merge", "(TestAccDnsARecord)"},
				{"TF_E2E_POSTGRES", "refs/pull/307/merge", "(TestAccPostgresqlFlexibleServer)"},
			},
		},
		{
			name:     "closed pr errors and triggers nothing",
			args:     []string{"pr", "308"},
			want:     nil,
			wantExit: 1,
		},
		{
			name: "explicit test regex overrides discovery",
			args: []string{"pr", "305", "TestAccPostgresqlFlexibleServer_complete"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/305/merge", "TestAccPostgresqlFlexibleServer_complete"}},
		},
		{
			name: "--all overrides the discovered regex with TestAcc",
			args: []string{"pr", "305", "--all"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/305/merge", "TestAcc"}},
		},
		{
			name: "--service filters a multi-service pr",
			args: []string{"pr", "307", "--service", "postgres"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/307/merge", "(TestAccPostgresqlFlexibleServer)"}},
		},
		{
			name: "changed untyped data source runs its derived test",
			args: []string{"pr", "309"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/309/merge", "(TestAccDataSourcePostgresqlflexibleServer)"}},
		},
		{
			name: "changed typed data source runs its derived test",
			args: []string{"pr", "310"},
			want: []trigger{{"TF_E2E_DNS", "refs/pull/310/merge", "(TestAccAzureRMDNSZoneDataSource)"}},
		},
		{
			// the same test file is discovered as CHANGED and DERIVED, and the
			// helper additionally traces the database resource — all deduped
			// into one build with the union regex
			name: "mixed pr dedupes changed, derived, and traced tests",
			args: []string{"pr", "311"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/311/merge", "(TestAccDataSourcePostgresqlflexibleServer|TestAccPostgresqlFlexibleServer|TestAccPostgresqlFlexibleServerDatabase)"}},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			scenario(t, "ast/azurerm", tt.name)
			gh := newMockGitHub(t, "testdata/azurerm", azurermASTPRs)
			tc := newMockTeamCity(t)
			clone := cloneUpstream(t, azurermUpstream)

			env := azurermEnv(gh, tc)
			env["TCTEST_LOCAL_REPO_PATH"] = clone
			maps.Copy(env, tt.extra)

			res := runTCTest(t, env, tt.args...)
			if res.exitCode != tt.wantExit {
				t.Fatalf("exit code = %d, want %d\noutput:\n%s", res.exitCode, tt.wantExit, res.output)
			}
			if !strings.Contains(res.output, "[AST]") {
				t.Fatalf("expected AST discovery mode to be used\noutput:\n%s", res.output)
			}
			assertTriggers(t, tc, res, tt.want)
		})
	}
}

// TestJSONOutput locks the --json machine-readable contract: stdout is a valid
// JSON array of triggered builds and nothing else.
func TestJSONOutput(t *testing.T) {
	t.Parallel()
	scenario(t, "output", "--json emits a valid JSON build array")
	gh := newMockGitHub(t, "testdata/azurerm", azurermPRs)
	tc := newMockTeamCity(t)

	res := runTCTest(t, azurermEnv(gh, tc), "pr", "201", "--json")
	if res.exitCode != 0 {
		t.Fatalf("exit code = %d, want 0\noutput:\n%s", res.exitCode, res.output)
	}

	var builds []struct {
		PR          int    `json:"pr"`
		Service     string `json:"service"`
		BuildNumber int    `json:"build_number"`
		URL         string `json:"url"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(res.output)), &builds); err != nil {
		t.Fatalf("--json output is not a bare JSON array: %v\noutput:\n%s", err, res.output)
	}
	if len(builds) != 1 || builds[0].PR != 201 || builds[0].Service != "postgres" || builds[0].BuildNumber == 0 || builds[0].URL == "" {
		t.Fatalf("unexpected build entries: %+v", builds)
	}
}

// TestQuietOutput locks the --quiet pr@service@build url line format.
func TestQuietOutput(t *testing.T) {
	t.Parallel()
	scenario(t, "output", "--quiet emits pr@service@build lines")
	gh := newMockGitHub(t, "testdata/azurerm", azurermPRs)
	tc := newMockTeamCity(t)

	res := runTCTest(t, azurermEnv(gh, tc), "pr", "201", "--quiet")
	if res.exitCode != 0 {
		t.Fatalf("exit code = %d, want 0\noutput:\n%s", res.exitCode, res.output)
	}

	lines := strings.Fields(strings.TrimSpace(res.output))
	quietRe := regexp.MustCompile(`^201@postgres@\d+$`)
	if len(lines) != 2 || !quietRe.MatchString(lines[0]) || !strings.Contains(lines[1], "viewQueued") {
		t.Fatalf("unexpected --quiet output: %q", res.output)
	}
}

// TestPrsCommand covers the prs command: enumerating open PRs and filtering
// them before discovery and triggering.
func TestPrsCommand(t *testing.T) {
	t.Parallel()

	openPRs := []listPR{
		{number: 220, author: "katbyte", labels: []string{"enhancement"}},
		{number: 221, author: "someone-else", labels: []string{"bug"}},
	}

	cases := []struct {
		name string
		args []string
		want []trigger
	}{
		{
			name: "author filter selects matching prs only",
			args: []string{"prs", "-a", "katbyte"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/220/merge", "(TestAccPostgresqlFlexibleServer)"}},
		},
		{
			name: "label filter selects matching prs only",
			args: []string{"prs", "-l", "bug"},
			want: []trigger{{"TF_E2E_DNS", "refs/pull/221/merge", "(TestAccDnsARecord)"}},
		},
		{
			name: "explicit regex applies to every matching pr",
			args: []string{"prs", "TestAccFoo_basic", "-a", "katbyte"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/220/merge", "TestAccFoo_basic"}},
		},
		{
			name: "no matching prs triggers nothing",
			args: []string{"prs", "-a", "nobody"},
			want: nil,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			scenario(t, "prs/azurerm", tt.name)
			gh := newMockGitHub(t, "testdata/azurerm", azurermPRs)
			gh.openPRs = openPRs
			tc := newMockTeamCity(t)

			res := runTCTest(t, azurermEnv(gh, tc), tt.args...)
			if res.exitCode != 0 {
				t.Fatalf("exit code = %d, want 0\noutput:\n%s", res.exitCode, res.output)
			}
			assertTriggers(t, tc, res, tt.want)
		})
	}
}

// awsASTPRs are served for the aws AST-mode tests; the merge refs for these
// numbers exist in the aws git fixture upstream.
var awsASTPRs = []prDef{
	{420, "open", "changed sdk resource without _resource suffix", []changedFile{
		{"internal/service/s3/bucket.go", "modified"},
	}},
	{421, "open", "changed framework resource", []changedFile{
		{"internal/service/rekognition/stream_processor.go", "modified"},
	}},
	{422, "open", "multiple services", []changedFile{
		{"internal/service/rekognition/collection.go", "modified"},
		{"internal/service/s3/bucket.go", "modified"},
	}},
	{423, "open", "generated and export files only", []changedFile{
		{"internal/service/rekognition/service_package_gen.go", "modified"},
		{"internal/service/rekognition/exports_test.go", "modified"},
		{"internal/service/rekognition/tags_gen.go", "modified"},
	}},
}

// TestASTDiscoveryAWS mirrors the aws API-mode cases through the local AST
// path: singular internal/service/ layout, framework and SDK-classic resources,
// generated files, and content-based unit-test classification (exports_test.go).
func TestASTDiscoveryAWS(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want []trigger
	}{
		{
			name: "sdk resource without _resource suffix derives its test family",
			args: []string{"pr", "420"},
			want: []trigger{{"TF_E2E", "refs/pull/420/merge", "(TestAccS3Bucket|TestAccS3BucketDataSource)"}},
		},
		{
			name: "framework resource derives its tests",
			args: []string{"pr", "421"},
			want: []trigger{{"TF_E2E", "refs/pull/421/merge", "(TestAccRekognitionStreamProcessor)"}},
		},
		{
			name: "multi-service pr triggers one build per service",
			args: []string{"pr", "422"},
			want: []trigger{
				{"TF_E2E", "refs/pull/422/merge", "(TestAccRekognitionCollection)"},
				{"TF_E2E", "refs/pull/422/merge", "(TestAccS3Bucket|TestAccS3BucketDataSource)"},
			},
		},
		{
			// exports_test.go is classified as a unit test here because AST mode
			// reads its content (no TestAcc funcs), unlike the API path's
			// filename-based fallback
			name: "generated and export files only trigger nothing",
			args: []string{"pr", "423"},
			want: nil,
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
			if res.exitCode != 0 {
				t.Fatalf("exit code = %d, want 0\noutput:\n%s", res.exitCode, res.output)
			}
			if !strings.Contains(res.output, "[AST]") {
				t.Fatalf("expected AST discovery mode to be used\noutput:\n%s", res.output)
			}
			assertTriggers(t, tc, res, tt.want)
		})
	}
}
