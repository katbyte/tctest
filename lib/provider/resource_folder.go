package provider

import (
	"path"
	"slices"
	"strings"
)

// ResourceFolderGroups are the directories below a service that hold one folder per resource in the resource-per-folder
// layout. Every file directly inside one of those folders belongs to that resource, whatever it is called:
//
//	internal/services/batch/resources/batch_account/{resource.go, r_create.go, r_schema.go, data_source.go, list.go, resource_test.go}
//	internal/services/managedredis/actions/managed_redis_flush_databases/{action.go, action_test.go}
var ResourceFolderGroups = []string{"resources", "actions"}

// resourceFolderOwnTests are the files in a resource folder that have tests of their own (data_source.go →
// data_source_test.go), so a change to one only needs those. A change to any other file in the folder (resource.go, r_*.go,
// shared models) runs every test in the folder.
var resourceFolderOwnTests = []string{"data_source.go", "list.go", "action.go", "ephemeral.go"}

// resourceFolderSharedTests are the test files in a resource folder that hold no tests, only setup the folder's tests share
// (test_helpers_test.go holds the configs they build on). They count as part of the resource, so a change to one runs every
// test in the folder.
var resourceFolderSharedTests = []string{"test_helpers_test.go"}

// InResourceFolder returns true if relPath sits directly inside a resource folder:
// internal/services/<service>/<group>/<folder>/<file>, where group is one of ResourceFolderGroups.
func InResourceFolder(relPath string) bool {
	return isResourceFolder(path.Dir(relPath))
}

// BelowResourceFolder returns true if relPath sits in a package nested inside a resource folder, like its migration/ package:
// internal/services/<service>/<group>/<folder>/<package>/<file>. The file belongs to that resource, but its tests sit up in
// the folder, not beside it, so it is never a resource file itself whatever the file regex says.
func BelowResourceFolder(relPath string) bool {
	for dir := path.Dir(path.Dir(relPath)); dir != "." && dir != "/"; dir = path.Dir(dir) {
		if isResourceFolder(dir) {
			return true
		}
	}
	return false
}

// isResourceFolder returns true if dir is a resource folder: internal/services/<service>/<group>/<folder>.
func isResourceFolder(dir string) bool {
	for _, prefix := range ServiceDirPrefixes {
		_, rest, found := strings.Cut("/"+dir, "/"+prefix+"/")
		if !found {
			continue
		}
		parts := strings.Split(rest, "/") // <service>/<group>/<folder>
		return len(parts) == 3 && slices.Contains(ResourceFolderGroups, parts[1])
	}
	return false
}

// isResourceFolderSharedTest returns true if f is a test file holding setup its resource folder's tests share, see
// resourceFolderSharedTests.
func (f *File) isResourceFolderSharedTest() bool {
	return slices.Contains(resourceFolderSharedTests, f.Name) && InResourceFolder(f.RelPath)
}

// resourceFolderPrefix is ResourcePrefix for a file in a resource folder. The folder already scopes the resource, so only
// the files with tests of their own keep a prefix ("data_source.go" → "data_source"); everything else returns "", which
// IsTestFor treats as every test file in the folder.
func (f *File) resourceFolderPrefix() string {
	if slices.Contains(resourceFolderOwnTests, f.Name) {
		return f.BaseName
	}
	return ""
}
