package cli

import (
	"fmt"
	"sort"

	"github.com/katbyte/tctest/lib/clog"
)

// ListServices lists all service directory names under internal/services/ in the repo
func (gr GithubRepo) ListServices() ([]string, error) {
	client, ctx := gr.NewClient()

	clog.Log.Debugf("listing services for %s/%s...", gr.Owner, gr.Name)
	_, dirContents, _, err := client.Repositories.GetContents(ctx, gr.Owner, gr.Name, "internal/services", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list services directory for %s/%s: %w", gr.Owner, gr.Name, err)
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
