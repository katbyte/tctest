package gh

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/google/go-github/v45/github"
	"github.com/katbyte/tctest/lib/clog"
)

func (r Repo) PrURL(pr int) string {
	return "https://github.com/" + r.Owner + "/" + r.Name + "/pull/" + strconv.Itoa(pr)
}

func (r Repo) ListAllPullRequests(state string, cb func([]*github.PullRequest, *github.Response) error) error {
	client, ctx := r.NewClient()

	opts := &github.PullRequestListOptions{
		State: state,
		ListOptions: github.ListOptions{
			Page:    1,
			PerPage: 100,
		},
	}

	for {
		clog.Log.Debugf("Listing all PRs for %s/%s (Page %d)...", r.Owner, r.Name, opts.ListOptions.Page)
		prs, resp, err := client.PullRequests.List(ctx, r.Owner, r.Name, opts)
		if err != nil {
			return fmt.Errorf("unable to list PRs for %s/%s (Page %d): %w", r.Owner, r.Name, opts.ListOptions.Page, err)
		}

		if err = cb(prs, resp); err != nil {
			return fmt.Errorf("callback failed for %s/%s (Page %d): %w", r.Owner, r.Name, opts.ListOptions.Page, err)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return nil
}

func (r Repo) GetAllPullRequests(state string) (*map[int]github.PullRequest, error) {
	m := map[int]github.PullRequest{}

	err := r.ListAllPullRequests(state, func(prs []*github.PullRequest, resp *github.Response) error {
		for i, p := range prs {
			if p == nil {
				clog.Log.Debugf("prs[%d] was nil, skipping", i)
				continue
			}

			n := p.GetNumber()
			if n == 0 {
				clog.Log.Debugf("prs[%d].Number was nil/0, skipping", i)
				continue
			}

			m[p.GetNumber()] = *p
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get all prs for %s/%s: %w", r.Owner, r.Name, err)
	}

	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	sorted := map[int]github.PullRequest{}

	for _, k := range keys {
		sorted[k] = m[k]
	}

	return &sorted, nil
}
