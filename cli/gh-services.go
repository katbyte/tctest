package cli

import (
	"fmt"
	"sort"

	"github.com/katbyte/tctest/lib/clog"
)

// ListServices lists all service directory names under internal/services/ in the repo
func (ghr GithubRepo) ListServices() ([]string, error) {
	client, ctx := ghr.NewClient()

	clog.Log.Debugf("listing services for %s/%s...", ghr.Owner, ghr.Name)
	_, dirContents, _, err := client.Repositories.GetContents(ctx, ghr.Owner, ghr.Name, "internal/services", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list services directory for %s/%s: %w", ghr.Owner, ghr.Name, err)
	}

	var services []string
	for _, entry := range dirContents {
		if entry.GetType() == "dir" {
			services = append(services, entry.GetName())
		}
	}

	sort.Strings(services)
	return services, nil
}
