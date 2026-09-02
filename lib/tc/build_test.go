package tc

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestXmlEscape(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		`Test(A|B)`:            `Test(A|B)`,
		`a&b`:                  `a&amp;b`,
		`<x>`:                  `&lt;x&gt;`,
		`say "hi"`:             `say &#34;hi&#34;`,
		`https://x?a=1&b=2`:    `https://x?a=1&amp;b=2`,
		`(TestAccFoo|TestBar)`: `(TestAccFoo|TestBar)`,
	}
	for in, want := range cases {
		if got := xmlEscape(in); got != want {
			t.Errorf("xmlEscape(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCheckFinishedBuild(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		body    string
		wantErr string // substring the error must contain, "" for no error
	}{
		"successful build": {
			body: `<build id="123" state="finished" status="SUCCESS"><statusText>Tests passed: 4</statusText></build>`,
		},
		"failed build is not cancelled": {
			body: `<build id="123" state="finished" status="FAILURE"><statusText>Tests failed: 1</statusText></build>`,
		},
		"cancelled with canceledInfo text": {
			body: `<build id="123" state="finished" status="UNKNOWN"><statusText>Canceled</statusText>` +
				`<canceledInfo timestamp="20200521T000000+0000"><text>Failed to collect changes: cannot find commit</text></canceledInfo></build>`,
			wantErr: "build 123 was cancelled: Failed to collect changes: cannot find commit",
		},
		"cancelled without canceledInfo text falls back to statusText": {
			body: `<build id="123" state="finished" status="UNKNOWN"><statusText>Canceled</statusText>` +
				`<canceledInfo timestamp="20200521T000000+0000"/></build>`,
			wantErr: "build 123 was cancelled: Canceled",
		},
		"cancelled with no reason at all": {
			body:    `<build id="123" state="finished" status="UNKNOWN"><canceledInfo/></build>`,
			wantErr: "build 123 was cancelled",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/app/rest/"+DefaultAPIVersion+"/builds/123" {
					http.NotFound(w, r)
					return
				}
				_, _ = fmt.Fprint(w, tc.body)
			}))
			defer ts.Close()

			token := "t"
			s := Server{Server: ts.URL, token: &token}

			err := s.CheckFinishedBuild(123)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("CheckFinishedBuild() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("CheckFinishedBuild() = nil, want error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("CheckFinishedBuild() = %q, want error containing %q", err, tc.wantErr)
			}
		})
	}
}
