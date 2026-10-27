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

func TestBelowResourceFolder(t *testing.T) {
	t.Parallel()

	for path, want := range map[string]bool{
		"internal/services/batch/resources/batch_account/migration/account_resource_v0_to_v1.go": true,
		"internal/services/managedredis/actions/managed_redis_flush_databases/helpers/flush.go":  true,

		"internal/services/batch/resources/batch_account/resource.go":   false, // in the folder, not below it
		"internal/services/storage/migration/storage_queue_v1_to_v2.go": false, // flat layout
		"internal/services/batch/resources/helpers.go":                  false, // in the group but not in a folder
		"internal/services/batch/validate/account_name.go":              false, // helper package beside the folders
	} {
		if got := BelowResourceFolder(path); got != want {
			t.Errorf("BelowResourceFolder(%s) = %v, want %v", path, got, want)
		}
	}
}

func TestBelowResourceFolderIsAHelper(t *testing.T) {
	t.Parallel()

	// a package nested in a resource folder is a helper of that resource whatever its files are called, so the folder's tests
	// are found through it rather than beside it
	for _, path := range []string{
		"internal/services/batch/resources/batch_account/migration/account_resource_v0_to_v1.go",
		"internal/services/notificationhub/resources/notification_hub/migration/notification_hub_resource.go", // the _resource.go rule would claim it
		"internal/services/appservice/resources/windows_web_app/migration/windows_web_app.go",
	} {
		if f := NewFile(path); f.Type != FileTypeHelper {
			t.Errorf("%s: type = %s, want [HELPER]", path, f.TypeLabel())
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
		{changed: "test_helpers_test.go", prefix: "", tests: folderTests}, // setup the folder's tests share
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

func TestResourceFolderSharedTestsOutsideAFolder(t *testing.T) {
	t.Parallel()

	// only a resource folder's test_helpers_test.go is setup for the folder; anywhere else it is an ordinary test file
	if f := NewFile("internal/services/batch/test_helpers_test.go"); f.Type != FileTypeTest {
		t.Errorf("type = %s, want [TEST]", f.TypeLabel())
	}
}

func TestMigrationResourcePrefix(t *testing.T) {
	t.Parallel()

	for path, want := range map[string]string{
		"internal/services/storage/migration/storage_queue_v1_to_v2.go":                                                 "storage_queue",
		"internal/services/appconfiguration/resources/app_configuration_feature/migration/feature_resource_v0_to_v1.go": "", // the folder names the resource
		"internal/services/notificationhub/resources/notification_hub/migration/notification_hub_resource.go":           "",
	} {
		f := NewFile(path)
		if got := f.MigrationResourcePrefix(); got != want {
			t.Errorf("MigrationResourcePrefix(%s) = %q, want %q", path, got, want)
		}
	}
}
