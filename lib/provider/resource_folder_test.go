package provider

import (
	"regexp"
	"slices"
	"testing"
)

func TestInResourceFolder(t *testing.T) {
	t.Parallel()

	for path, want := range map[string]bool{
		"internal/services/batch/resources/batch_account/resource.go":                    true,
		"internal/services/batch/resources/batch_account/r_create.go":                    true,
		"internal/services/batch/resources/batch_account/resource_test.go":               true,
		"internal/services/managedredis/actions/managed_redis_flush_databases/action.go": true,
		"internal/service/ec2/resources/vpc/resource.go":                                 true,

		"internal/services/batch/batch_account_resource.go":                     false, // flat layout
		"internal/services/batch/validate/account_name.go":                      false, // helper package
		"internal/services/logic/parse/action.go":                               false, // helper package that happens to hold an action.go
		"internal/services/batch/account/resource.go":                           false, // one level down, not in a group
		"internal/services/batch/resources/helpers.go":                          false, // in the group but not in a folder
		"internal/services/batch/resources/batch_account/migration/v0_to_v1.go": false, // below the folder
		"internal/acceptance/resources/thing/resource.go":                       false, // not in a service
	} {
		if got := InResourceFolder(path); got != want {
			t.Errorf("InResourceFolder(%s) = %v, want %v", path, got, want)
		}
	}
}

func TestResourceFolderTests(t *testing.T) {
	t.Parallel()

	suffixes := []*regexp.Regexp{
		regexp.MustCompile(`^_resource.*_test$`),
		regexp.MustCompile(`^_test$`),
		regexp.MustCompile(`^_list_test$`),
		regexp.MustCompile(`^_identity_gen_test$`),
		regexp.MustCompile(`^_data_source_test$`),
	}
	const folder = "internal/services/batch/resources/batch_account/"
	folderTests := []string{"data_source_test", "list_test", "resource_identity_gen_test", "resource_test"}

	for _, tc := range []struct {
		changed string
		prefix  string
		tests   []string
	}{
		{changed: "resource.go", prefix: "", tests: folderTests},
		{changed: "r_update.go", prefix: "", tests: folderTests},
		{changed: "batch_account.go", prefix: "", tests: folderTests},
		{changed: "data_source.go", prefix: "data_source", tests: []string{"data_source_test"}},
		{changed: "list.go", prefix: "list", tests: []string{"list_test"}},
	} {
		f := NewFile(folder + tc.changed)
		if f.Type != FileTypeResource {
			t.Errorf("%s: type = %s, want [RESOURCE]", tc.changed, f.TypeLabel())
		}
		if got := f.ResourcePrefix(); got != tc.prefix {
			t.Errorf("%s: ResourcePrefix() = %q, want %q", tc.changed, got, tc.prefix)
		}

		var got []string
		for _, name := range folderTests {
			test := NewFile(folder + name + ".go")
			if test.IsTestFor([]string{f.ResourcePrefix()}, suffixes) {
				got = append(got, name)
			}
		}
		if !slices.Equal(got, tc.tests) {
			t.Errorf("%s: matched tests %v, want %v", tc.changed, got, tc.tests)
		}
	}
}
