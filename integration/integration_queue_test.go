package integration

import (
	"slices"
	"strings"
	"testing"
)

// nightlyQueue mirrors a TeamCity queue shared by several projects' nightly runs.
var nightlyQueue = []queuedBuild{
	{901, "Mau_GoogleBeta_ServiceSweeper", "Service Sweeper", "Development / Mau's Project / Google Beta / Nightly Tests", "nightly-test"},
	{902, "Mau_GlobalSweepers_Folder", "Folder Sweeper", "Development / Mau's Project / Global Sweepers", "nightly-test"},
	{903, "Kt_Google_Datafusion", "Datafusion - Acceptance Tests", "Development / Kt's Project / Google / Nightly Tests", "nightly-test"},
	{904, "Mau_Google_Datalineage", "Datalineage - Acceptance Tests", "Development / Mau's Project / Google / Nightly Tests", ""},
}

// TestQueueCommand covers listing the TeamCity build queue and removing the builds whose full build configuration name
// matches a regex.
func TestQueueCommand(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        []string
		wantExit    int
		wantOutput  []string
		notOutput   []string
		wantCancels []int
	}{
		{
			name: "queue lists every queued build",
			args: []string{"queue"},
			wantOutput: []string{
				"found 4 queued builds",
				"901 Development / Mau's Project / Google Beta / Nightly Tests / Service Sweeper [nightly-test]",
				"903 Development / Kt's Project / Google / Nightly Tests / Datafusion - Acceptance Tests [nightly-test]",
				"904 Development / Mau's Project / Google / Nightly Tests / Datalineage - Acceptance Tests\n",
			},
		},
		{
			name:       "list filters on the full build configuration name",
			args:       []string{"queue", "list", "Mau's Project / Google"},
			wantOutput: []string{"2 match Mau's Project / Google", "901 ", "904 "},
			notOutput:  []string{"902 ", "903 "},
		},
		{
			name:       "list regex is case-sensitive unless (?i) is used",
			args:       []string{"queue", "ls", "(?i)mau.*sweeper"},
			wantOutput: []string{"2 match", "901 ", "902 "},
			notOutput:  []string{"903 ", "904 "},
		},
		{
			name:       "list --quiet prints id@buildtypeid url lines",
			args:       []string{"queue", "list", "Kt", "--quiet"},
			wantOutput: []string{"903@Kt_Google_Datafusion http://"},
			notOutput:  []string{"found", "901"},
		},
		{
			name:        "remove --force removes only the matching builds",
			args:        []string{"queue", "remove", "Mau", "--force"},
			wantOutput:  []string{"removed 901", "removed 902", "removed 904", "removed 3 of 3 matching queued builds"},
			notOutput:   []string{"removed 903"},
			wantCancels: []int{901, 902, 904},
		},
		{
			name:       "remove --dry-run removes nothing",
			args:       []string{"queue", "rm", "Mau", "--dry-run"},
			wantOutput: []string{"3 match Mau", "[DRY RUN] would remove 3 queued builds"},
		},
		{
			name:       "remove without --force aborts when there is no confirmation",
			args:       []string{"queue", "remove", "Mau"},
			wantExit:   1,
			wantOutput: []string{"remove 3 queued builds matching \"Mau\"? [y/N]", "aborted, no queued builds were removed (use --force to skip confirmation)"},
		},
		{
			name:       "remove matching nothing succeeds",
			args:       []string{"queue", "remove", "Nobody's Project", "--force"},
			wantOutput: []string{"0 match", "no queued builds to remove"},
		},
		{
			name:       "remove rejects an empty regex",
			args:       []string{"queue", "remove", ""},
			wantExit:   1,
			wantOutput: []string{"regex can't be empty"},
		},
		{
			name:       "remove rejects an invalid regex",
			args:       []string{"queue", "remove", "Mau(", "--force"},
			wantExit:   1,
			wantOutput: []string{"invalid regex", "Mau(", "missing closing )"},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			scenario(t, "queue", tt.name)
			tc := newMockTeamCity(t, withQueue(nightlyQueue))

			res := runTCTest(t, map[string]string{"TCTEST_SERVER": tc.srv.URL, "NO_COLOR": "1"}, tt.args...)
			if res.exitCode != tt.wantExit {
				t.Fatalf("exit code = %d, want %d\noutput:\n%s", res.exitCode, tt.wantExit, res.output)
			}
			for _, want := range tt.wantOutput {
				if !strings.Contains(res.output, want) {
					t.Errorf("output missing %q", want)
				}
			}
			for _, not := range tt.notOutput {
				if strings.Contains(res.output, not) {
					t.Errorf("output unexpectedly contains %q", not)
				}
			}
			if t.Failed() {
				t.Fatalf("output:\n%s", res.output)
			}

			cancels := tc.Cancels()
			ids := make([]int, 0, len(cancels))
			for _, c := range cancels {
				ids = append(ids, c.ID)
				if want := `removed from queue by tctest (matched "Mau")`; c.Comment != want {
					t.Errorf("build %d removed with comment %q, want %q", c.ID, c.Comment, want)
				}
			}
			if !slices.Equal(ids, tt.wantCancels) {
				t.Fatalf("removed builds = %v, want %v\noutput:\n%s", ids, tt.wantCancels, res.output)
			}
			assertTriggers(t, tc, res, nil)
		})
	}
}
