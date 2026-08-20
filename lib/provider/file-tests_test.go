package provider

import (
	"reflect"
	"testing"
)

const (
	testNamePrefix = "TestAccAWSInstance"
	testNameKms    = "TestAccAWSInstance_RootBlockDevice_KmsKeyArn"
)

const testFileContent = `package example

import "testing"

func TestAccAWSInstance_basic(t *testing.T) {
	t.Log("basic")
}

// TestAccAWSInstance_RootBlockDevice_KmsKeyArn tests the kms key arn
func TestAccAWSInstance_RootBlockDevice_KmsKeyArn(t *testing.T) {
	t.Log("kms")
	t.Log("more")
}

func testAccCheckInstanceExists(t *testing.T) {
	t.Log("helper")
}

func TestAccAWSInstance_disappears(t *testing.T) {
	t.Log("disappears")
}
`

func newTestFile(t *testing.T, patch string) *File {
	t.Helper()
	f := NewFile("internal/service/ec2/instance_test.go")
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
			// lines 10-11 are inside TestAccAWSInstance_RootBlockDevice_KmsKeyArn (doc comment starts line 9)
			patch:  "@@ -10,2 +10,2 @@\n-	t.Log(\"kms\")\n+	t.Log(\"kms!\")",
			want:   []string{testNameKms},
			wantOk: true,
		},
		"change to doc comment counts": {
			patch:  "@@ -9,1 +9,1 @@",
			want:   []string{testNameKms},
			wantOk: true,
		},
		"changes across two tests": {
			patch:  "@@ -5,3 +5,3 @@\n@@ -19,2 +19,2 @@",
			want:   []string{"TestAccAWSInstance_basic", "TestAccAWSInstance_disappears"},
			wantOk: true,
		},
		"change only to non-test helper": {
			patch:  "@@ -15,3 +15,3 @@",
			want:   nil,
			wantOk: true,
		},
		"hunk with no count (single line)": {
			patch:  "@@ -6 +6 @@",
			want:   []string{"TestAccAWSInstance_basic"},
			wantOk: true,
		},
		"pure deletion still marks position": {
			patch:  "@@ -12,1 +12,0 @@",
			want:   []string{testNameKms},
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
