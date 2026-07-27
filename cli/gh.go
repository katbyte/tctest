package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/katbyte/tctest/lib/clog"
	"github.com/katbyte/tctest/lib/gh"
	"github.com/katbyte/tctest/lib/provider"
)

// wrap the common gh lib shared with my other tools. splits common GH code from this CLI tool's specific tooling code
type GithubRepo struct {
	gh.Repo
}

func (f FlagData) NewRepo() GithubRepo {
	ownerrepo := f.GH.Repo

	parts := strings.Split(ownerrepo, "/")
	if len(parts) != 2 {
		panic("repo was not in the format owner/repo")
	}
	owner, repo := parts[0], parts[1]

	token := f.GH.Token
	clog.Log.Debugf("new gh: %s@%s/%s", maskToken(token), owner, repo)

	return GithubRepo{gh.NewRepo(owner, repo, token)}
}

func maskToken(token string) string {
	if len(token) <= 4 {
		return "****"
	}
	return token[:4] + "****"
}

// ListServices lists all service directory names under the provider's service
// directory (internal/services/ or internal/service/).
func (ghr GithubRepo) ListServices() ([]string, error) {
	client, ctx := ghr.NewClient()

	for _, prefix := range provider.ServiceDirPrefixes {
		clog.Log.Debugf("listing services for %s/%s at %s...", ghr.Owner, ghr.Name, prefix)
		_, dirContents, resp, err := client.Repositories.GetContents(ctx, ghr.Owner, ghr.Name, prefix, nil)
		if err != nil {
			if resp != nil && resp.StatusCode == 404 {
				clog.Log.Debugf("  %s not found, trying next prefix", prefix)
				continue
			}
			return nil, fmt.Errorf("failed to list services directory for %s/%s at %s: %w", ghr.Owner, ghr.Name, prefix, err)
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

	return nil, fmt.Errorf("no service directory found for %s/%s (tried %s)", ghr.Owner, ghr.Name, strings.Join(provider.ServiceDirPrefixes, ", "))
}
