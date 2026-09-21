package tc

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
)

func TestQueuedBuildFullName(t *testing.T) {
	t.Parallel()

	const name, project = "Service Sweeper", "Development / Mau's Project / Google Beta / Nightly Tests"

	cases := map[string]struct {
		build QueuedBuild
		want  string
	}{
		"project and name": {
			build: QueuedBuild{BuildTypeID: "X", Name: name, ProjectName: project},
			want:  project + " / " + name,
		},
		"no project": {
			build: QueuedBuild{BuildTypeID: "X", Name: name},
			want:  name,
		},
		"no name falls back to the build type id": {
			build: QueuedBuild{BuildTypeID: "X", ProjectName: "Development"},
			want:  "Development / X",
		},
	}

	for desc, tc := range cases {
		if got := tc.build.FullName(); got != tc.want {
			t.Errorf("%s: FullName() = %q, want %q", desc, got, tc.want)
		}
	}
}

func TestGetBuildQueue(t *testing.T) {
	t.Parallel()

	// with a page size of 2, the second page repeats build 2 as if the queue shifted between requests
	pages := map[string]string{
		"start:0,count:2": `<builds count="2">` +
			`<build id="1" buildTypeId="A" branchName="nightly-test" webUrl="https://tc/build/1"><buildType name="Service Sweeper" projectName="Development / Mau's Project"/></build>` +
			`<build id="2" buildTypeId="B" webUrl="https://tc/build/2"><buildType name="Folder Sweeper" projectName="Development / Global Sweepers"/></build>` +
			`</builds>`,
		"start:2,count:2": `<builds count="2">` +
			`<build id="2" buildTypeId="B" webUrl="https://tc/build/2"><buildType name="Folder Sweeper" projectName="Development / Global Sweepers"/></build>` +
			`<build id="3" buildTypeId="C" webUrl="https://tc/build/3"><buildType name="Datafusion" projectName="Development / Mau's Project"/></build>` +
			`</builds>`,
		"start:4,count:2": `<builds count="1">` +
			`<build id="4" buildTypeId="D" webUrl="https://tc/build/4"><buildType name="Datalineage" projectName="Development / Mau's Project"/></build>` +
			`</builds>`,
	}

	var mu sync.Mutex
	var requested []string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/app/rest/"+DefaultAPIVersion+"/buildQueue" {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("fields"); got != queueFields {
			t.Errorf("fields = %q, want %q", got, queueFields)
		}

		locator := r.URL.Query().Get("locator")
		mu.Lock()
		requested = append(requested, locator)
		mu.Unlock()

		page, ok := pages[locator]
		if !ok {
			http.Error(w, "unexpected locator "+locator, http.StatusBadRequest)
			return
		}
		_, _ = fmt.Fprint(w, page)
	}))
	defer ts.Close()

	const mausProject = "Development / Mau's Project"

	builds, err := NewServerUsingTokenAuth(ts.URL, "t").getBuildQueuePaged(2)
	if err != nil {
		t.Fatalf("getBuildQueuePaged() error: %v", err)
	}

	want := []QueuedBuild{
		{ID: 1, BuildTypeID: "A", Name: "Service Sweeper", ProjectName: mausProject, Branch: "nightly-test", URL: "https://tc/build/1"},
		{ID: 2, BuildTypeID: "B", Name: "Folder Sweeper", ProjectName: "Development / Global Sweepers", URL: "https://tc/build/2"},
		{ID: 3, BuildTypeID: "C", Name: "Datafusion", ProjectName: mausProject, URL: "https://tc/build/3"},
		{ID: 4, BuildTypeID: "D", Name: "Datalineage", ProjectName: mausProject, URL: "https://tc/build/4"},
	}
	if !slices.Equal(builds, want) {
		t.Errorf("getBuildQueuePaged() =\n  %+v\nwant\n  %+v", builds, want)
	}

	mu.Lock()
	defer mu.Unlock()
	if wantRequested := []string{"start:0,count:2", "start:2,count:2", "start:4,count:2"}; !slices.Equal(requested, wantRequested) {
		t.Errorf("requested locators = %v, want %v", requested, wantRequested)
	}
}

func TestGetBuildQueueEmpty(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `<builds count="0"/>`)
	}))
	defer ts.Close()

	builds, err := NewServerUsingTokenAuth(ts.URL, "t").GetBuildQueue()
	if err != nil {
		t.Fatalf("GetBuildQueue() error: %v", err)
	}
	if len(builds) != 0 {
		t.Errorf("GetBuildQueue() = %+v, want no builds", builds)
	}
}

func TestGetBuildQueueHTTPError(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Access denied.\nYou do not have enough permissions", http.StatusForbidden)
	}))
	defer ts.Close()

	_, err := NewServerUsingTokenAuth(ts.URL, "t").GetBuildQueue()
	if err == nil {
		t.Fatal("GetBuildQueue() = nil error, want an error")
	}
	if want := "HTTP status NOT OK: 403 listing build queue: Access denied. You do not have enough permissions"; err.Error() != want {
		t.Errorf("GetBuildQueue() error = %q, want %q", err, want)
	}
}

func TestCancelQueuedBuild(t *testing.T) {
	t.Parallel()

	const comment = `removed from queue by tctest (matched "Mau's <Project> & co")`

	cases := map[string]struct {
		cancelStatus int
		cancelBody   string
		stateStatus  int // 0 when the state must not be looked up
		state        string
		wantErr      string // substring the error must contain, "" for no error
		wantNotQueue bool
	}{
		"removed": {
			cancelStatus: http.StatusOK,
			cancelBody:   `<build id="123" state="finished"/>`,
		},
		"started running before it was removed": {
			cancelStatus: http.StatusBadRequest,
			cancelBody:   "Build is not queued",
			stateStatus:  http.StatusOK,
			state:        "running",
			wantErr:      "build 123 is running: build is no longer queued",
			wantNotQueue: true,
		},
		"no longer exists": {
			cancelStatus: http.StatusNotFound,
			stateStatus:  http.StatusNotFound,
			wantErr:      "build 123 no longer exists: build is no longer queued",
			wantNotQueue: true,
		},
		"still queued but refused": {
			cancelStatus: http.StatusForbidden,
			cancelBody:   "Access denied",
			stateStatus:  http.StatusOK,
			state:        "queued",
			wantErr:      "HTTP status NOT OK: 403 removing build 123 from the queue: Access denied",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/app/rest/"+DefaultAPIVersion+"/buildQueue/id:123":
					b, err := io.ReadAll(r.Body)
					if err != nil {
						t.Errorf("reading cancel request body: %v", err)
					}
					var req struct {
						XMLName        xml.Name `xml:"buildCancelRequest"`
						Comment        string   `xml:"comment,attr"`
						ReaddIntoQueue string   `xml:"readdIntoQueue,attr"`
					}
					if err := xml.Unmarshal(b, &req); err != nil {
						t.Errorf("cancel request body %q is not a buildCancelRequest: %v", b, err)
					}
					if req.Comment != comment || req.ReaddIntoQueue != "false" {
						t.Errorf("cancel request = %+v, want comment %q and readdIntoQueue false", req, comment)
					}
					w.WriteHeader(tc.cancelStatus)
					_, _ = fmt.Fprint(w, tc.cancelBody)

				case r.Method == http.MethodGet && r.URL.Path == "/app/rest/"+DefaultAPIVersion+"/builds/123/state":
					if tc.stateStatus == 0 {
						t.Error("build state looked up after a successful removal")
					}
					w.WriteHeader(tc.stateStatus)
					_, _ = fmt.Fprint(w, tc.state)

				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer ts.Close()

			err := NewServerUsingTokenAuth(ts.URL, "t").CancelQueuedBuild(123, comment)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("CancelQueuedBuild() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("CancelQueuedBuild() = nil, want error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("CancelQueuedBuild() = %q, want error containing %q", err, tc.wantErr)
			}
			if got := errors.Is(err, ErrNotQueued); got != tc.wantNotQueue {
				t.Errorf("errors.Is(err, ErrNotQueued) = %t, want %t", got, tc.wantNotQueue)
			}
		})
	}
}
