package gh

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/google/go-github/v89/github"
	"github.com/katbyte/tctest/lib/clog"
	"github.com/katbyte/tctest/lib/git"
)

// GitHub API pull request states, as returned by PullRequest.GetState()
// and accepted by ListAllPullRequests/GetAllPullRequests.
const (
	PRStateOpen   = "open"
	PRStateClosed = "closed"
)

func (r Repo) PrURL(pr int) string {
	return "https://github.com/" + r.Owner + "/" + r.Name + "/pull/" + strconv.Itoa(pr)
}

func (r Repo) CloneURL() string {
	return "https://github.com/" + r.Owner + "/" + r.Name + ".git"
}

// CheckoutPR fetches the merge ref for a PR and checks out FETCH_HEAD in the given repo path.
// Returns the short SHA of the checked-out merge commit.
func (Repo) CheckoutPR(repoPath string, prNumber int) (string, error) {
	if err := git.FetchPRMergeRef(repoPath, prNumber); err != nil {
		return "", fmt.Errorf("failed to fetch PR merge ref: %w", err)
	}
	sha, err := git.CheckoutFetchHead(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to checkout merge commit: %w", err)
	}
	return sha, nil
}

// mergeableAttempts and mergeableRetryDelay control how long GetPrForBuild waits for
// GitHub to finish computing a PR's mergeability (the computation is asynchronous and
// a GET on the PR is what kicks it off).
const (
	mergeableAttempts   = 5
	mergeableRetryDelay = 3 * time.Second
)

// GetPrForBuild fetches a PR and verifies builds can be triggered on its merge ref: it
// must be open, mergeable, and have a merge commit SHA. Mergeable has to be checked
// explicitly — GitHub keeps returning a stale merge_commit_sha for a PR that has
// *become* conflicted, so a nil-SHA check alone lets conflicted PRs through to trigger
// builds on a stale or missing refs/pull/N/merge ref, which then fail (or silently test
// outdated code) inside TeamCity where nobody sees it.
func (r Repo) GetPrForBuild(number int) (*github.PullRequest, error) {
	client, ctx := r.NewClient()

	for attempt := 1; ; attempt++ {
		clog.Log.Debugf("fetching data for PR %s/%s/#%d...", r.Owner, r.Name, number)
		pr, _, err := client.PullRequests.Get(ctx, r.Owner, r.Name, number)
		if err != nil {
			return nil, WrapGitHubError(err, fmt.Sprintf("fetching PR %s/%s/#%d", r.Owner, r.Name, number))
		}

		clog.Log.Debugf("  checking pr state: %v", pr.GetState())
		if pr.GetState() == PRStateClosed {
			return nil, errors.New("cannot start build for a closed pr")
		}

		if pr.Mergeable == nil {
			if attempt < mergeableAttempts {
				clog.Log.Debugf("  mergeability of PR #%d not yet computed by github, retrying (%d/%d)...", number, attempt, mergeableAttempts)
				time.Sleep(mergeableRetryDelay)
				continue
			}
			return nil, fmt.Errorf("github has not finished computing mergeability for PR #%d, try again shortly", number)
		}

		if !pr.GetMergeable() {
			return nil, fmt.Errorf("PR #%d has merge conflicts that must be resolved before tests can be run", number)
		}

		if pr.MergeCommitSHA == nil {
			return nil, fmt.Errorf("PR #%d is mergeable but github has no merge commit SHA for it yet, try again shortly", number)
		}

		return pr, nil
	}
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
		clog.Log.Debugf("Listing all PRs for %s/%s (Page %d)...", r.Owner, r.Name, opts.Page)
		prs, resp, err := client.PullRequests.List(ctx, r.Owner, r.Name, opts)
		if err != nil {
			return WrapGitHubError(err, fmt.Sprintf("listing PRs for %s/%s (Page %d)", r.Owner, r.Name, opts.Page))
		}

		if err = cb(prs, resp); err != nil {
			return fmt.Errorf("callback failed for %s/%s (Page %d): %w", r.Owner, r.Name, opts.Page, err)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return nil
}

func (r Repo) GetAllPullRequests(state string) (*[]github.PullRequest, error) {
	var allPRs []github.PullRequest

	if err := r.ListAllPullRequests(state, func(prs []*github.PullRequest, _ *github.Response) error {
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

			allPRs = append(allPRs, *p)
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to get all prs for %s/%s: %w", r.Owner, r.Name, err)
	}

	slices.SortFunc(allPRs, func(a, b github.PullRequest) int {
		return cmp.Compare(a.GetNumber(), b.GetNumber())
	})

	return &allPRs, nil
}

func (r Repo) ListAllPullRequestFiles(pri int, cb func([]*github.CommitFile, *github.Response) error) error {
	client, ctx := r.NewClient()

	opts := &github.ListOptions{
		Page:    1,
		PerPage: 100,
	}

	for {
		clog.Log.Debugf("Listing all files for %s/%s/pull/%d (Page %d)...", r.Owner, r.Name, pri, opts.Page)
		files, resp, err := client.PullRequests.ListFiles(ctx, r.Owner, r.Name, pri, opts)
		if err != nil {
			return WrapGitHubError(err, fmt.Sprintf("listing files for %s/%s/pull/%d (Page %d)", r.Owner, r.Name, pri, opts.Page))
		}

		if err = cb(files, resp); err != nil {
			return fmt.Errorf("callback failed for %s/%s/pull/%d (Page %d): %w", r.Owner, r.Name, pri, opts.Page, err)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return nil
}
