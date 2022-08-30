package gh

import (
	"fmt"

	"github.com/google/go-github/v45/github"
	"github.com/katbyte/tctest/lib/clog"
)

func (r Repo) GetLabelsFor(number int) (*[]string, error) {
	client, ctx := r.NewClient()

	opts := &github.ListOptions{
		Page:    1,
		PerPage: 100,
	}

	allLabels := []string{}
	for {
		clog.Log.Debugf("Listing labels for %s/%s/%d (Page %d)...", r.Owner, r.Name, number, opts.Page)
		labels, resp, err := client.Issues.ListLabelsByIssue(ctx, r.Owner, r.Name, number, opts)
		if err != nil {
			return nil, fmt.Errorf("unable to list PRs for %s/%s (Page %d): %w", r.Owner, r.Name, opts.Page, err)
		}

		for _, l := range labels {
			allLabels = append(allLabels, l.GetName())
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return &allLabels, nil
}
