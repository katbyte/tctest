package integration

import (
	"encoding/json"
	"maps"
	"regexp"
	"strings"
	"testing"
)

// azurermPRs defines the PRs the mock GitHub serves for the azurerm fixture.
var azurermPRs = []prDef{
	{1001, "open", "changed test file", []changedFile{
		{"internal/services/postgres/postgresql_flexible_server_resource_test.go", "modified"},
	}},
	{1002, "open", "changed resource file", []changedFile{
		{"internal/services/postgres/postgresql_flexible_server_resource.go", "modified"},
	}},
	{1003, "open", "changed cross-package helper only", []changedFile{
		{"internal/services/postgres/validate/database_charset.go", "modified"},
	}},
	{1004, "open", "multiple services", []changedFile{
		{"internal/services/postgres/postgresql_flexible_server_resource_test.go", "modified"},
		{"internal/services/dns/dns_a_record_resource.go", "modified"},
	}},
	{1005, "closed", "closed pr", []changedFile{
		{"internal/services/postgres/postgresql_flexible_server_resource_test.go", "modified"},
	}},
	{1006, "open", "changed untyped data source", []changedFile{
		{"internal/services/postgres/postgresql_flexible_server_data_source.go", "modified"},
	}},
	{1007, "open", "changed typed data source", []changedFile{
		{"internal/services/dns/dns_zone_data_source.go", "modified"},
	}},
	{1008, "open", "mixed resource, test, and helper", []changedFile{
		{"internal/services/postgres/postgresql_flexible_server_resource.go", "modified"},
		{"internal/services/postgres/postgresql_flexible_server_resource_test.go", "modified"},
		{"internal/services/postgres/validate/database_charset.go", "modified"},
	}},
	{1020, "open", "postgres improvement", []changedFile{
		{"internal/services/postgres/postgresql_flexible_server_resource_test.go", "modified"},
	}},
	{1021, "open", "dns bug fix", []changedFile{
		{"internal/services/dns/dns_a_record_resource.go", "modified"},
	}},
}

// azurermASTPRs are served for the AST-mode tests; the merge refs for these
// numbers exist in the git fixture upstream.
var azurermASTPRs = []prDef{
	{2001, "open", "same-package helper changed", []changedFile{
		{"internal/services/dns/ipv6_address.go", "modified"},
	}},
	{2002, "open", "cross-package helper changed", []changedFile{
		{"internal/services/postgres/validate/database_charset.go", "modified"},
	}},
	{2003, "open", "two-level helper chain changed", []changedFile{
		{"internal/services/postgres/parse/postgresql_aad_administrator.go", "modified"},
	}},
	{2004, "open", "vendored dependency changed", []changedFile{
		{"vendor/github.com/hashicorp/go-azure-sdk/resource-manager/cosmosdb/2024-08-15/cosmosdb/client.go", "modified"},
	}},
	{2005, "open", "changed test file", []changedFile{
		{"internal/services/postgres/postgresql_flexible_server_resource_test.go", "modified"},
	}},
	{2006, "open", "changed resource file", []changedFile{
		{"internal/services/postgres/postgresql_flexible_server_resource.go", "modified"},
	}},
	{2007, "open", "multiple services", []changedFile{
		{"internal/services/postgres/postgresql_flexible_server_resource_test.go", "modified"},
		{"internal/services/dns/dns_a_record_resource.go", "modified"},
	}},
	{2008, "closed", "closed pr", []changedFile{
		{"internal/services/postgres/postgresql_flexible_server_resource_test.go", "modified"},
	}},
	{2009, "open", "changed untyped data source", []changedFile{
		{"internal/services/postgres/postgresql_flexible_server_data_source.go", "modified"},
	}},
	{2010, "open", "changed typed data source", []changedFile{
		{"internal/services/dns/dns_zone_data_source.go", "modified"},
	}},
	{2011, "open", "mixed resource, test, and helper", []changedFile{
		{"internal/services/postgres/postgresql_flexible_server_resource.go", "modified"},
		{"internal/services/postgres/postgresql_flexible_server_resource_test.go", "modified"},
		{"internal/services/postgres/validate/database_charset.go", "modified"},
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
			args: []string{"pr", "1001"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/1001/merge", "(TestAccPostgresqlFlexibleServer)"}},
		},
		{
			name: "changed resource file runs derived sibling tests",
			args: []string{"pr", "1002"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/1002/merge", "(TestAccDataSourcePostgresqlflexibleServer|TestAccPostgresqlFlexibleServer)"}},
		},
		{
			name: "helper-only change discovers nothing in API mode",
			args: []string{"pr", "1003"},
			want: nil,
		},
		{
			name: "multi-service pr triggers one build per service",
			args: []string{"pr", "1004"},
			want: []trigger{
				{"TF_E2E_DNS", "refs/pull/1004/merge", "(TestAccDnsARecord)"},
				{"TF_E2E_POSTGRES", "refs/pull/1004/merge", "(TestAccPostgresqlFlexibleServer)"},
			},
		},
		{
			name:     "closed pr errors and triggers nothing",
			args:     []string{"pr", "1005"},
			want:     nil,
			wantExit: 1,
		},
		{
			name: "explicit test regex overrides discovery",
			args: []string{"pr", "1001", "TestAccPostgresqlFlexibleServer_complete"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/1001/merge", "TestAccPostgresqlFlexibleServer_complete"}},
		},
		{
			name: "--all overrides the discovered regex with TestAcc",
			args: []string{"pr", "1001", "--all"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/1001/merge", "TestAcc"}},
		},
		{
			name: "--service filters a multi-service pr",
			args: []string{"pr", "1004", "--service", "postgres"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/1004/merge", "(TestAccPostgresqlFlexibleServer)"}},
		},
		{
			name: "changed untyped data source runs its derived test",
			args: []string{"pr", "1006"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/1006/merge", "(TestAccDataSourcePostgresqlflexibleServer)"}},
		},
		{
			name: "changed typed data source runs its derived test",
			args: []string{"pr", "1007"},
			want: []trigger{{"TF_E2E_DNS", "refs/pull/1007/merge", "(TestAccAzureRMDNSZoneDataSource)"}},
		},
		{
			// helpers don't trace in API mode, so only the resource's tests run;
			// the AST-mode mirror of this PR also picks up the traced database tests
			name: "mixed pr runs changed and derived tests only",
			args: []string{"pr", "1008"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/1008/merge", "(TestAccDataSourcePostgresqlflexibleServer|TestAccPostgresqlFlexibleServer)"}},
		},
		{
			name: "--add-tests appends to the discovered regex",
			args: []string{"pr", "1002", "--add-tests", "TestAccExtraThing"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/1002/merge", "(TestAccDataSourcePostgresqlflexibleServer|TestAccPostgresqlFlexibleServer|TestAccExtraThing)"}},
		},
		{
			name:     "--max-builds-per-pr limit errors and triggers nothing",
			args:     []string{"pr", "1004", "--max-builds-per-pr", "1"},
			want:     nil,
			wantExit: 1,
		},
		{
			name: "--force bypasses the max-builds-per-pr limit",
			args: []string{"pr", "1004", "--max-builds-per-pr", "1", "--force"},
			want: []trigger{
				{"TF_E2E_DNS", "refs/pull/1004/merge", "(TestAccDnsARecord)"},
				{"TF_E2E_POSTGRES", "refs/pull/1004/merge", "(TestAccPostgresqlFlexibleServer)"},
			},
		},
		{
			name: "--dry-run triggers nothing and exits cleanly",
			args: []string{"pr", "1001", "--dry-run"},
			want: nil,
		},
		{
			name: "--service all triggers every service directly",
			args: []string{"pr", "1001", "--service", "all", "--all"},
			want: []trigger{
				{"TF_E2E_COSMOS", "refs/pull/1001/merge", "TestAcc"},
				{"TF_E2E_DNS", "refs/pull/1001/merge", "TestAcc"},
				{"TF_E2E_POSTGRES", "refs/pull/1001/merge", "TestAcc"},
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
			args:  []string{"pr", "2001"},
			extra: map[string]string{"TCTEST_FILEREGEX": narrowFileRegex},
			want:  []trigger{{"TF_E2E_DNS", "refs/pull/2001/merge", "(TestAccDataSourceDnsAAAARecord|TestAccDnsAAAARecord)"}},
		},
		{
			name: "cross-package helper traces through import symbol usage",
			args: []string{"pr", "2002"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/2002/merge", "(TestAccPostgresqlFlexibleServerDatabase)"}},
		},
		{
			name: "two-level helper chain traces only symbol users",
			args: []string{"pr", "2003"},
			// the virtual endpoint resource imports the same migration package but
			// uses an upgrader from a different file — it must NOT be selected
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/2003/merge", "(TestAccPostgreSQLAdministrator)"}},
		},
		{
			name: "vendored dependency change traces to importing resources",
			args: []string{"pr", "2004"},
			want: []trigger{{"TF_E2E_COSMOS", "refs/pull/2004/merge", "(TestAccCosmosDBAccount|TestAccDataSourceCosmosDBAccount)"}},
		},

		// the simple discovery cases covered by the API-mode tests, mirrored
		// here to prove the local AST path handles them identically
		{
			name: "changed test file runs its tests",
			args: []string{"pr", "2005"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/2005/merge", "(TestAccPostgresqlFlexibleServer)"}},
		},
		{
			name: "changed resource file runs derived sibling tests",
			args: []string{"pr", "2006"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/2006/merge", "(TestAccDataSourcePostgresqlflexibleServer|TestAccPostgresqlFlexibleServer)"}},
		},
		{
			name: "multi-service pr triggers one build per service",
			args: []string{"pr", "2007"},
			want: []trigger{
				{"TF_E2E_DNS", "refs/pull/2007/merge", "(TestAccDnsARecord)"},
				{"TF_E2E_POSTGRES", "refs/pull/2007/merge", "(TestAccPostgresqlFlexibleServer)"},
			},
		},
		{
			name:     "closed pr errors and triggers nothing",
			args:     []string{"pr", "2008"},
			want:     nil,
			wantExit: 1,
		},
		{
			name: "explicit test regex overrides discovery",
			args: []string{"pr", "2005", "TestAccPostgresqlFlexibleServer_complete"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/2005/merge", "TestAccPostgresqlFlexibleServer_complete"}},
		},
		{
			name: "--all overrides the discovered regex with TestAcc",
			args: []string{"pr", "2005", "--all"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/2005/merge", "TestAcc"}},
		},
		{
			name: "--service filters a multi-service pr",
			args: []string{"pr", "2007", "--service", "postgres"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/2007/merge", "(TestAccPostgresqlFlexibleServer)"}},
		},
		{
			name: "changed untyped data source runs its derived test",
			args: []string{"pr", "2009"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/2009/merge", "(TestAccDataSourcePostgresqlflexibleServer)"}},
		},
		{
			name: "changed typed data source runs its derived test",
			args: []string{"pr", "2010"},
			want: []trigger{{"TF_E2E_DNS", "refs/pull/2010/merge", "(TestAccAzureRMDNSZoneDataSource)"}},
		},
		{
			// the same test file is discovered as CHANGED and DERIVED, and the
			// helper additionally traces the database resource — all deduped
			// into one build with the union regex
			name: "mixed pr dedupes changed, derived, and traced tests",
			args: []string{"pr", "2011"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/2011/merge", "(TestAccDataSourcePostgresqlflexibleServer|TestAccPostgresqlFlexibleServer|TestAccPostgresqlFlexibleServerDatabase)"}},
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
			if !strings.Contains(res.output, "[mode=AST]") {
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

	res := runTCTest(t, azurermEnv(gh, tc), "pr", "1001", "--json")
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
	if len(builds) != 1 || builds[0].PR != 1001 || builds[0].Service != "postgres" || builds[0].BuildNumber == 0 || builds[0].URL == "" {
		t.Fatalf("unexpected build entries: %+v", builds)
	}
}

// TestQuietOutput locks the --quiet pr@service@build url line format.
func TestQuietOutput(t *testing.T) {
	t.Parallel()
	scenario(t, "output", "--quiet emits pr@service@build lines")
	gh := newMockGitHub(t, "testdata/azurerm", azurermPRs)
	tc := newMockTeamCity(t)

	res := runTCTest(t, azurermEnv(gh, tc), "pr", "1001", "--quiet")
	if res.exitCode != 0 {
		t.Fatalf("exit code = %d, want 0\noutput:\n%s", res.exitCode, res.output)
	}

	lines := strings.Fields(strings.TrimSpace(res.output))
	quietRe := regexp.MustCompile(`^1001@postgres@\d+$`)
	if len(lines) != 2 || !quietRe.MatchString(lines[0]) || !strings.Contains(lines[1], "viewQueued") {
		t.Fatalf("unexpected --quiet output: %q", res.output)
	}
}

// TestPrsCommand covers the prs command: enumerating open PRs and filtering
// them before discovery and triggering.
func TestPrsCommand(t *testing.T) {
	t.Parallel()

	openPRs := []listPR{
		{number: 1020, author: "katbyte", labels: []string{"enhancement"}},
		{number: 1021, author: "someone-else", labels: []string{"bug"}},
	}

	cases := []struct {
		name string
		args []string
		want []trigger
	}{
		{
			name: "author filter selects matching prs only",
			args: []string{"prs", "-a", "katbyte"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/1020/merge", "(TestAccPostgresqlFlexibleServer)"}},
		},
		{
			name: "label filter selects matching prs only",
			args: []string{"prs", "-l", "bug"},
			want: []trigger{{"TF_E2E_DNS", "refs/pull/1021/merge", "(TestAccDnsARecord)"}},
		},
		{
			name: "explicit regex applies to every matching pr",
			args: []string{"prs", "TestAccFoo_basic", "-a", "katbyte"},
			want: []trigger{{"TF_E2E_POSTGRES", "refs/pull/1020/merge", "TestAccFoo_basic"}},
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

// TestServicePackageProperty covers --properties-service-package / TCTEST_PROPERTIES_SERVICE_PACKAGE: the discovered
// service package name is sent as a build property, named SERVICE_PACKAGE by default, renameable, and disableable
// with an empty value (via the flag; an empty env var falls back to the default as viper treats it as unset).
func TestServicePackageProperty(t *testing.T) {
	t.Parallel()

	t.Run("default sends SERVICE_PACKAGE per service", func(t *testing.T) {
		t.Parallel()
		scenario(t, "api/azurerm", "properties-service-package default")
		gh := newMockGitHub(t, "testdata/azurerm", azurermPRs)
		tc := newMockTeamCity(t)

		res := runTCTest(t, azurermEnv(gh, tc), "pr", "1004")
		if res.exitCode != 0 {
			t.Fatalf("exit code = %d, want 0\noutput:\n%s", res.exitCode, res.output)
		}

		props := tc.Properties()
		if len(props) != 2 {
			t.Fatalf("expected 2 triggered builds, got %d\noutput:\n%s", len(props), res.output)
		}
		got := map[string]bool{}
		for _, p := range props {
			got[p["SERVICE_PACKAGE"]] = true
		}
		if !got["postgres"] || !got["dns"] {
			t.Fatalf("expected SERVICE_PACKAGE properties for postgres and dns, got %v\noutput:\n%s", got, res.output)
		}
	})

	t.Run("env renames the property", func(t *testing.T) {
		t.Parallel()
		scenario(t, "api/azurerm", "properties-service-package renamed")
		gh := newMockGitHub(t, "testdata/azurerm", azurermPRs)
		tc := newMockTeamCity(t)

		env := azurermEnv(gh, tc)
		env["TCTEST_PROPERTIES_SERVICE_PACKAGE"] = "PKG"

		res := runTCTest(t, env, "pr", "1001")
		if res.exitCode != 0 {
			t.Fatalf("exit code = %d, want 0\noutput:\n%s", res.exitCode, res.output)
		}

		props := tc.Properties()
		if len(props) != 1 {
			t.Fatalf("expected 1 triggered build, got %d\noutput:\n%s", len(props), res.output)
		}
		if got := props[0]["PKG"]; got != "postgres" {
			t.Fatalf("expected PKG=postgres, got %q\noutput:\n%s", got, res.output)
		}
		if v, ok := props[0]["SERVICE_PACKAGE"]; ok {
			t.Fatalf("expected no SERVICE_PACKAGE property when renamed, got %q\noutput:\n%s", v, res.output)
		}
	})

	t.Run("empty flag disables the property", func(t *testing.T) {
		t.Parallel()
		scenario(t, "api/azurerm", "properties-service-package disabled")
		gh := newMockGitHub(t, "testdata/azurerm", azurermPRs)
		tc := newMockTeamCity(t)

		res := runTCTest(t, azurermEnv(gh, tc), "pr", "1001", "--properties-service-package", "")
		if res.exitCode != 0 {
			t.Fatalf("exit code = %d, want 0\noutput:\n%s", res.exitCode, res.output)
		}

		props := tc.Properties()
		if len(props) != 1 {
			t.Fatalf("expected 1 triggered build, got %d\noutput:\n%s", len(props), res.output)
		}
		if v, ok := props[0]["SERVICE_PACKAGE"]; ok {
			t.Fatalf("expected no SERVICE_PACKAGE property, got %q\noutput:\n%s", v, res.output)
		}
	})
}
