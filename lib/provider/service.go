package provider

import "strings"

// ExtractService extracts the service name from a file path.
// Handles both /services/ (Azure) and /service/ (AWS) layouts.
// Returns empty string for non-service paths.
func (f File) ExtractService() string {
	for _, sep := range []string{"/services/", "/service/"} {
		parts := strings.Split(f.Path, sep)
		if len(parts) == 2 {
			return strings.Split(parts[1], "/")[0]
		}
	}
	return ""
}

// IsServicePath returns true if the path is within a service directory.
func (f File) IsServicePath() bool {
	return strings.Contains(f.Path, "/services/") || strings.Contains(f.Path, "/service/")
}
