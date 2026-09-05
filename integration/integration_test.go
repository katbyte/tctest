// Package integration contains integration tests: they run the real tctest binary against mock GitHub and
// TeamCity servers, using fixture provider repositories under testdata/.
//
// The binary is built once in TestMain. Each test spins up its own mock
// servers, runs the real binary with a fully-controlled environment, and
// asserts on the build-trigger requests TeamCity received.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/katbyte/tctest/lib/tc"
)

const mergeSHA = "0123456789abcdef0123456789abcdef01234567"

var (
	binPath         string // the tctest binary built from the repo root
	azurermUpstream string // git repo built from testdata/azurerm with refs/pull/N/merge refs
	awsUpstream     string // git repo built from testdata/aws with refs/pull/N/merge refs
	harnessHome     string // empty HOME for subprocesses so no real ~/.tctest or gitconfig leaks in
)

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "tctest-integration-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration test setup: %v\n", err)
		os.Exit(1)
	}

	code := func() int {
		harnessHome = filepath.Join(tmp, "home")
		if err := os.MkdirAll(harnessHome, 0o750); err != nil {
			fmt.Fprintf(os.Stderr, "integration test setup: %v\n", err)
			return 1
		}

		// build the real binary once, race-instrumented so a data race in the CLI surfaces as a test failure (the
		// race runtime prints a report and exits non-zero, which the exit-code and output assertions catch)
		binPath = filepath.Join(tmp, "tctest")
		build := exec.CommandContext(context.Background(), "go", "build", "-race", "-o", binPath, ".") //nolint:gosec // building the repo's own binary into a temp dir
		build.Dir = ".."                                                                               // tests run with CWD = the test package dir
		if out, err := build.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "integration test setup: building tctest: %v\n%s\n", err, out)
			return 1
		}

		// build the git upstreams used by the AST-mode tests
		azurermUpstream = filepath.Join(tmp, "azurerm-upstream")
		if err := buildGitUpstream("testdata/azurerm", azurermUpstream, 2000, 2020); err != nil {
			fmt.Fprintf(os.Stderr, "integration test setup: git fixture: %v\n", err)
			return 1
		}
		awsUpstream = filepath.Join(tmp, "aws-upstream")
		if err := buildGitUpstream("testdata/aws", awsUpstream, 20000, 20020); err != nil {
			fmt.Fprintf(os.Stderr, "integration test setup: git fixture: %v\n", err)
			return 1
		}

		fmt.Println("==> integration: running the tctest binary against mock GitHub + TeamCity servers...")
		return m.Run()
	}()

	_ = os.RemoveAll(tmp)
	os.Exit(code)
}

// --- git fixtures ---

func runGit(dir string, args ...string) error {
	base := make([]string, 0, 8+len(args))
	base = append(base,
		"-c", "user.name=integration-test", "-c", "user.email=integration-test@test.invalid",
		"-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main",
	)
	cmd := exec.CommandContext(context.Background(), "git", append(base, args...)...) //nolint:gosec // fixture setup with internally-constructed args
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return nil
}

// buildGitUpstream copies a fixture tree into dst, commits it, and points
// refs/pull/N/merge at HEAD for every N in [prFrom, prTo] so tctest's
// `git fetch origin pull/N/merge` works against it as a local remote.
func buildGitUpstream(src, dst string, prFrom, prTo int) error {
	if err := copyTree(src, dst); err != nil {
		return err
	}
	if err := runGit(dst, "init", "-q"); err != nil {
		return err
	}
	if err := runGit(dst, "add", "-A", "-f"); err != nil { // -f: the fixture vendor/ dir must be committed
		return err
	}
	if err := runGit(dst, "commit", "-q", "-m", "fixture"); err != nil {
		return err
	}
	for n := prFrom; n <= prTo; n++ {
		if err := runGit(dst, "update-ref", fmt.Sprintf("refs/pull/%d/merge", n), "HEAD"); err != nil {
			return err
		}
	}
	return nil
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		b, err := os.ReadFile(path) //nolint:gosec // fixture files
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644) //nolint:gosec // fixture files
	})
}

// cloneUpstream produces a fresh working clone for one test; tctest's AST mode
// fetches and checks out PR merge refs in it, so clones are never shared.
func cloneUpstream(t *testing.T, upstream string) string {
	t.Helper()
	parent := t.TempDir()
	dst := filepath.Join(parent, "repo")
	if err := runGit(parent, "clone", "-q", upstream, dst); err != nil {
		t.Fatalf("cloning fixture upstream: %v", err)
	}
	return dst
}

// --- mock GitHub (API + raw downloads) ---

type changedFile struct {
	path   string
	status string // "modified", "added", "removed"
	patch  string // unified diff served in the files listing, used by --individual test discovery
}

type prDef struct {
	number int
	state  string // "open" or "closed"
	title  string
	files  []changedFile
}

// listPR describes an entry served by the open-PR list endpoint (the prs command);
// the PR's state, title, and changed files still come from the matching prDef.
type listPR struct {
	number int
	author string
	labels []string
	draft  bool
}

type mockGitHub struct {
	srv        *httptest.Server
	fixture    string // fixture tree that backs raw downloads and contents listings
	prs        map[int]prDef
	openPRs    []listPR     // served by GET /repos/{o}/{r}/pulls for the prs command
	conflicted map[int]bool // PRs served with mergeable=false (and, like real GitHub, a stale merge_commit_sha)
}

// mockGitHubOption configures a mockGitHub before its server starts. The handler goroutines read these fields
// without locking, so they must be fully populated before httptest.NewServer spawns them — setting them on the
// returned struct afterwards is a data race the race detector flags even though the binary under test has not
// connected yet.
type mockGitHubOption func(*mockGitHub)

// withConflicted marks the given PRs as mergeable=false.
func withConflicted(conflicted map[int]bool) mockGitHubOption {
	return func(m *mockGitHub) { m.conflicted = conflicted }
}

// withOpenPRs sets the list served by GET /repos/{o}/{r}/pulls for the prs command.
func withOpenPRs(openPRs []listPR) mockGitHubOption {
	return func(m *mockGitHub) { m.openPRs = openPRs }
}

func newMockGitHub(t *testing.T, fixtureDir string, prs []prDef, opts ...mockGitHubOption) *mockGitHub {
	t.Helper()
	m := &mockGitHub{fixture: fixtureDir, prs: map[int]prDef{}}
	for _, pr := range prs {
		m.prs[pr.number] = pr
	}
	for _, opt := range opts {
		opt(m)
	}
	m.srv = httptest.NewServer(http.HandlerFunc(m.handle))
	t.Cleanup(m.srv.Close)
	return m
}

func (m *mockGitHub) apiURL() string { return m.srv.URL }
func (m *mockGitHub) rawURL() string { return m.srv.URL + "/raw" }

func (m *mockGitHub) handle(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

	// go-github's WithEnterpriseURLs appends /api/v3/ to the configured base URL, so API requests
	// arrive as /api/v3/repos/... — strip the prefix before routing
	if len(parts) >= 2 && parts[0] == "api" && parts[1] == "v3" {
		parts = parts[2:]
	}

	switch {
	// /raw/{owner}/{repo}/{ref}/{path...} — raw file downloads
	case parts[0] == "raw" && len(parts) >= 5:
		rel := strings.Join(parts[4:], "/")
		b, err := os.ReadFile(filepath.Join(m.fixture, filepath.FromSlash(rel))) //nolint:gosec // fixture reads
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(b) //nolint:gosec // G705: fixture bytes served to the binary under test, not a browser

	// /repos/{owner}/{repo}/...
	case parts[0] == "repos" && len(parts) >= 4:
		m.handleAPI(w, parts[3:])

	default:
		jsonNotFound(w)
	}
}

func (m *mockGitHub) handleAPI(w http.ResponseWriter, rest []string) {
	switch {
	// pulls — list open PRs (single page)
	case len(rest) == 1 && rest[0] == "pulls":
		out := make([]map[string]any, 0, len(m.openPRs))
		for _, pr := range m.openPRs {
			def := m.prs[pr.number]
			labels := make([]map[string]any, 0, len(pr.labels))
			for _, l := range pr.labels {
				labels = append(labels, map[string]any{"name": l})
			}
			out = append(out, map[string]any{
				"number": pr.number,
				"state":  def.state,
				"title":  def.title,
				"user":   map[string]any{"login": pr.author},
				"labels": labels,
				"draft":  pr.draft,
			})
		}
		writeJSON(w, out)

	// pulls/{n}
	case len(rest) == 2 && rest[0] == "pulls":
		n, err := strconv.Atoi(rest[1])
		pr, ok := m.prs[n]
		if err != nil || !ok {
			jsonNotFound(w)
			return
		}
		// merge_commit_sha stays populated even when mergeable is false — real GitHub
		// keeps serving the stale test-merge SHA for a PR that has become conflicted
		writeJSON(w, map[string]any{
			"number":           pr.number,
			"state":            pr.state,
			"title":            pr.title,
			"mergeable":        !m.conflicted[n],
			"merge_commit_sha": mergeSHA,
		})

	// pulls/{n}/files
	case len(rest) == 3 && rest[0] == "pulls" && rest[2] == "files":
		n, err := strconv.Atoi(rest[1])
		pr, ok := m.prs[n]
		if err != nil || !ok {
			jsonNotFound(w)
			return
		}
		files := make([]map[string]any, 0, len(pr.files))
		for _, f := range pr.files {
			entry := map[string]any{"filename": f.path, "status": f.status}
			if f.patch != "" {
				entry["patch"] = f.patch
			}
			files = append(files, entry)
		}
		writeJSON(w, files)

	// contents/{path...} — directory listings
	case len(rest) >= 2 && rest[0] == "contents":
		rel := strings.Join(rest[1:], "/")
		entries, err := os.ReadDir(filepath.Join(m.fixture, filepath.FromSlash(rel)))
		if err != nil {
			jsonNotFound(w)
			return
		}
		out := make([]map[string]any, 0, len(entries))
		for _, e := range entries {
			typ := "file"
			if e.IsDir() {
				typ = "dir"
			}
			out = append(out, map[string]any{"name": e.Name(), "path": rel + "/" + e.Name(), "type": typ})
		}
		writeJSON(w, out)

	default:
		jsonNotFound(w)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func jsonNotFound(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(`{"message":"Not Found"}`))
}

// --- mock TeamCity ---

type trigger struct {
	BuildTypeID string
	Branch      string
	TestPattern string
}

type mockTeamCity struct {
	srv *httptest.Server

	mu       sync.Mutex
	triggers []trigger
	props    []map[string]string // all properties of each trigger, parallel to triggers
	nextID   int
}

func newMockTeamCity(t *testing.T) *mockTeamCity {
	t.Helper()
	m := &mockTeamCity{nextID: 714000}
	m.srv = httptest.NewServer(http.HandlerFunc(m.handle))
	t.Cleanup(m.srv.Close)
	return m
}

func (m *mockTeamCity) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/app/rest/"+tc.DefaultAPIVersion+"/buildQueue" {
		http.NotFound(w, r)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req struct {
		BuildType struct {
			ID string `xml:"id,attr"`
		} `xml:"buildType"`
		Properties struct {
			Property []struct {
				Name  string `xml:"name,attr"`
				Value string `xml:"value,attr"`
			} `xml:"property"`
		} `xml:"properties"`
	}
	if err := xml.Unmarshal(body, &req); err != nil {
		http.Error(w, fmt.Sprintf("bad build xml: %v", err), http.StatusBadRequest)
		return
	}

	props := map[string]string{}
	for _, p := range req.Properties.Property {
		props[p.Name] = p.Value
	}

	m.mu.Lock()
	m.nextID++
	id := m.nextID
	m.triggers = append(m.triggers, trigger{
		BuildTypeID: req.BuildType.ID,
		Branch:      props["teamcity.build.branch"],
		TestPattern: props["TEST_PATTERN"],
	})
	m.props = append(m.props, props)
	m.mu.Unlock()

	w.Header().Set("Content-Type", "application/xml")
	_, _ = fmt.Fprintf(w, `<build id="%d"/>`, id)
}

func (m *mockTeamCity) Triggers() []trigger {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]trigger, len(m.triggers))
	copy(out, m.triggers)
	return out
}

// Properties returns the full property set of each triggered build, in trigger order.
func (m *mockTeamCity) Properties() []map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]map[string]string, len(m.props))
	copy(out, m.props)
	return out
}

// --- binary runner ---

type runResult struct {
	output   string // combined stdout + stderr
	exitCode int
}

// runTCTest executes the built tctest binary with a fully-controlled
// environment: nothing from the caller's shell (real TCTEST_* vars, tokens,
// ~/.tctest, gitconfig) can leak into the run.
func runTCTest(t *testing.T, env map[string]string, args ...string) runResult {
	t.Helper()

	full := map[string]string{
		"PATH":                 os.Getenv("PATH"), // git must be findable
		"HOME":                 harnessHome,
		"TMPDIR":               os.TempDir(),
		"TCTEST_TOKEN_TC":      "integration-test-token",
		"TCTEST_BUILD_TYPE_ID": "TF_E2E",
	}
	maps.Copy(full, env)
	envSlice := make([]string, 0, len(full))
	for k, v := range full {
		envSlice = append(envSlice, k+"="+v)
	}

	cmd := exec.CommandContext(context.Background(), binPath, args...) //nolint:gosec // running the binary under test
	cmd.Dir = harnessHome                                              // no .tctest config in CWD either
	cmd.Env = envSlice

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	res := runResult{output: out.String()}
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			res.exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("running tctest %v: %v\n%s", args, err, out.String())
		}
	}
	return res
}

// scenario prints a one-line confirmation when a subtest passes, so plain
// `go test ./...` output shows which end-to-end scenarios actually ran.
func scenario(t *testing.T, kind, name string) {
	t.Helper()
	t.Cleanup(func() {
		if !t.Failed() && !t.Skipped() {
			fmt.Printf("      ok  integration [%s] %s\n", kind, name)
		}
	})
}

// assertTriggers compares the builds TeamCity received against want,
// ignoring order (service iteration order is not defined).
func assertTriggers(t *testing.T, mock *mockTeamCity, res runResult, want []trigger) {
	t.Helper()

	got := mock.Triggers()
	byKey := func(s []trigger) {
		slices.SortFunc(s, func(a, b trigger) int {
			if c := strings.Compare(a.BuildTypeID, b.BuildTypeID); c != 0 {
				return c
			}
			return strings.Compare(a.TestPattern, b.TestPattern)
		})
	}
	byKey(got)
	byKey(want)

	fail := len(got) != len(want)
	if !fail {
		for i := range got {
			if got[i] != want[i] {
				fail = true
				break
			}
		}
	}
	if fail {
		t.Fatalf("triggered builds mismatch\n got: %+v\nwant: %+v\n\ntctest output:\n%s", got, want, res.output)
	}
}
