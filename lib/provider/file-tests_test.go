package provider

import (
	"reflect"
	"testing"
)

const (
	testNamePrefix   = "TestAccPostgresqlFlexibleServer"
	testNameComplete = "TestAccPostgresqlFlexibleServer_complete"
)

const testFileContent = `package example

import "testing"

func TestAccPostgresqlFlexibleServer_basic(t *testing.T) {
	t.Log("basic")
}

// TestAccPostgresqlFlexibleServer_complete tests the complete configuration
func TestAccPostgresqlFlexibleServer_complete(t *testing.T) {
	t.Log("complete")
	t.Log("more")
}

func testAccCheckPostgresqlFlexibleServerExists(t *testing.T) {
	t.Log("helper")
}

func TestAccPostgresqlFlexibleServer_requiresImport(t *testing.T) {
	t.Log("import")
}
`

func newTestFile(t *testing.T, patch string) *File {
	t.Helper()
	f := NewFile("internal/services/postgres/postgresql_flexible_server_resource_test.go")
	f.SetContent([]byte(testFileContent))
	if patch != "" {
		f.SetPatch(patch)
	}
	return &f
}

func TestExtractTests(t *testing.T) {
	t.Parallel()

	f := newTestFile(t, "")
	got, err := f.ExtractTests("_", false)
	if err != nil {
		t.Fatalf("ExtractTests() error: %v", err)
	}
	want := []string{testNamePrefix, testNamePrefix, testNamePrefix}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExtractTests() = %v, want %v", got, want)
	}
}

func TestExtractChangedTests(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		patch  string
		want   []string
		wantOk bool
	}{
		"no patch falls back": {
			patch:  "",
			want:   nil,
			wantOk: false,
		},
		"change inside one test": {
			// lines 10-11 are inside TestAccPostgresqlFlexibleServer_complete (doc comment starts line 9)
			patch:  "@@ -10,2 +10,2 @@\n-	t.Log(\"kms\")\n+	t.Log(\"kms!\")",
			want:   []string{testNameComplete},
			wantOk: true,
		},
		"change to doc comment counts": {
			patch:  "@@ -9,1 +9,1 @@",
			want:   []string{testNameComplete},
			wantOk: true,
		},
		"changes across two tests": {
			patch:  "@@ -5,3 +5,3 @@\n@@ -19,2 +19,2 @@",
			want:   []string{"TestAccPostgresqlFlexibleServer_basic", "TestAccPostgresqlFlexibleServer_requiresImport"},
			wantOk: true,
		},
		"change only to non-test helper": {
			patch:  "@@ -15,3 +15,3 @@",
			want:   nil,
			wantOk: true,
		},
		"hunk with no count (single line)": {
			patch:  "@@ -6 +6 @@",
			want:   []string{"TestAccPostgresqlFlexibleServer_basic"},
			wantOk: true,
		},
		"pure deletion still marks position": {
			patch:  "@@ -12,1 +12,0 @@",
			want:   []string{testNameComplete},
			wantOk: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newTestFile(t, tc.patch)
			got, ok, err := f.ExtractChangedTests()
			if err != nil {
				t.Fatalf("ExtractChangedTests() error: %v", err)
			}
			if ok != tc.wantOk {
				t.Fatalf("ExtractChangedTests() ok = %v, want %v", ok, tc.wantOk)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ExtractChangedTests() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDiscoverTests(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		patch      string
		individual bool
		want       []string
	}{
		"individual off splits to prefixes": {
			patch:      "@@ -10,2 +10,2 @@",
			individual: false,
			want:       []string{testNamePrefix, testNamePrefix, testNamePrefix},
		},
		"individual with patch yields modified full names": {
			patch:      "@@ -10,2 +10,2 @@",
			individual: true,
			want:       []string{testNameComplete},
		},
		"individual without patch falls back to prefixes": {
			patch:      "",
			individual: true,
			want:       []string{testNamePrefix, testNamePrefix, testNamePrefix},
		},
		"individual with helper-only diff falls back to prefixes": {
			patch:      "@@ -15,3 +15,3 @@",
			individual: true,
			want:       []string{testNamePrefix, testNamePrefix, testNamePrefix},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newTestFile(t, tc.patch)
			got, err := f.DiscoverTests("_", false, tc.individual)
			if err != nil {
				t.Fatalf("DiscoverTests() error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("DiscoverTests() = %v, want %v", got, tc.want)
			}
		})
	}
}
