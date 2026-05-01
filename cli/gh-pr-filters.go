package cli

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-github/v45/github"
	"github.com/katbyte/tctest/lib/cout"
)

type Filter struct {
	Name string
	PR   func(github.PullRequest) (bool, error) // todo shjould this return an error?
}

func (f FlagData) GetFilters() ([]Filter, error) {
	var filters []Filter

	// should these return errors
	if f := GetFilterForAuthors(f.GH.FilterPRs.Authors); f != nil {
		filters = append(filters, *f)
	}

	if f := GetFilterForLabelsOr(f.GH.FilterPRs.LabelsOr); f != nil {
		filters = append(filters, *f)
	}

	if f := GetFilterForLabelsAnd(f.GH.FilterPRs.LabelsAnd); f != nil {
		filters = append(filters, *f)
	}

	if f := GetFilterForMilestone(f.GH.FilterPRs.Milestone); f != nil {
		filters = append(filters, *f)
	}

	if f := GetFilterForCreatedTime(f.GH.FilterPRs.CreationTime); f != nil {
		filters = append(filters, *f)
	}

	if f := GetFilterForUpdatedTime(f.GH.FilterPRs.UpdatedTime); f != nil {
		filters = append(filters, *f)
	}

	titleFilter, err := GetFilterForTitleRegex(f.GH.FilterPRs.TitleRegex)
	if err != nil {
		return nil, err
	}
	if titleFilter != nil {
		filters = append(filters, *titleFilter)
	}

	cout.Printf("\n")

	return filters, nil
}

func GetFilterForAuthors(authors []string) *Filter {
	if len(authors) == 0 {
		return nil
	}

	authorMap := map[string]bool{}
	for _, a := range authors {
		authorMap[a] = true
	}

	cout.Printf("  authors: <magenta>%s</>\n", strings.Join(authors, "</>,<magenta>"))

	return &Filter{
		Name: "authors",
		PR: func(pr github.PullRequest) (bool, error) {
			author := pr.User.GetLogin()

			if _, ok := authorMap[author]; ok {
				cout.Printf("    author: <green>%s</>\n", author)
				return true, nil
			}
			cout.Printf("    author: <red>%s</>\n", author)

			return false, nil
		},
	}
}

func GetFilterForMilestone(milestoneRaw string) *Filter {
	if milestoneRaw == "" {
		return nil
	}
	filterMilestone := strings.TrimPrefix(milestoneRaw, "-")
	negate := strings.HasPrefix(milestoneRaw, "-")

	cout.Printf("  milestone: <magenta>%s</>\n", milestoneRaw)

	return &Filter{
		Name: "milestones",
		PR: func(pr github.PullRequest) (bool, error) {
			milestone := pr.GetMilestone().GetTitle()

			//nolint:gocritic
			if strings.EqualFold(filterMilestone, milestone) && !negate {
				cout.Printf("    milestone: <green>%s</> <gray>(%s)</>\n", filterMilestone, milestone)
				return true, nil
			} else if negate {
				cout.Printf("    milestone: <green>-%s</> <gray>(%s)</>\n", filterMilestone, milestone)
				return true, nil
			} else {
				//revive:disable:indent-error-flow
				cout.Printf("    milestone: <red>%s</> <gray>(%s)</>\n", filterMilestone, milestone)
				return false, nil
			}
		},
	}
}

func GetFilterForCreatedTime(duration time.Duration) *Filter {
	if duration == time.Nanosecond {
		return nil
	}
	cutoffTime := time.Now().Add(-duration)

	cout.Printf("  created within: <magenta>%s</>\n", duration.String())

	return &Filter{
		Name: "creation-time",
		PR: func(pr github.PullRequest) (bool, error) {
			createdAt := pr.GetCreatedAt()

			if createdAt.After(cutoffTime) {
				cout.Printf("    created: <green>%s</> <gray>(%s)</>\n", createdAt.Format(time.RFC822), cutoffTime.Format(time.RFC822))
				return true, nil
			}

			cout.Printf("    created: <red>%s</> <gray>(%s)</>\n", createdAt.Format(time.RFC822), cutoffTime.Format(time.RFC822))

			return false, nil
		},
	}
}

func GetFilterForUpdatedTime(duration time.Duration) *Filter {
	if duration == time.Nanosecond {
		return nil
	}
	cutoffTime := time.Now().Add(-duration)

	cout.Printf("  updated within: <magenta>%s</>\n", duration.String())

	return &Filter{
		Name: "creation-time",
		PR: func(pr github.PullRequest) (bool, error) {
			createdAt := pr.GetUpdatedAt()

			if createdAt.After(cutoffTime) {
				cout.Printf("    updated: <green>%s</> <gray>(%s)</>\n", createdAt.Format(time.RFC822), cutoffTime.Format(time.RFC822))
				return true, nil
			}

			cout.Printf("    updated: <red>%s</> <gray>(%s)</>\n", createdAt.Format(time.RFC822), cutoffTime.Format(time.RFC822))

			return false, nil
		},
	}
}

func GetFilterForLabelsOr(labels []string) *Filter {
	return GetFilterForLabels(labels, false)
}

func GetFilterForLabelsAnd(labels []string) *Filter {
	return GetFilterForLabels(labels, true)
}

func GetFilterForLabels(labels []string, and bool) *Filter {
	if len(labels) == 0 {
		return nil
	}

	filterLabelMap := map[string]bool{}
	for _, l := range labels {
		filterLabelMap[strings.TrimPrefix(l, "-")] = strings.HasPrefix(l, "-")
	}

	action := "or"
	actionAnd := false
	if and {
		action = "and"
		actionAnd = true
	}

	cout.Printf("  labels %s: <blue>%s</>\n", action, strings.Join(labels, "</>,<blue>"))

	//	found := false
	return &Filter{
		Name: "labels " + action,
		PR: func(pr github.PullRequest) (bool, error) {
			labelMap := map[string]bool{}
			for _, l := range pr.Labels {
				// todo check for emvy label name?
				labelMap[l.GetName()] = true // casing?
			}

			// and
			// for each label,

			if actionAnd {
				cout.Printf("    labels all:")
			} else {
				cout.Printf("    labels any:")
			}

			andFail := false
			orPass := false

			// for each filter label see if it exists
			for filterLabel, negate := range filterLabelMap {
				_, found := labelMap[filterLabel]

				//nolint:gocritic
				if found && !negate {
					orPass = true
					cout.Printf(" <green>%s</>", filterLabel)
				} else if found && negate {
					andFail = true
					cout.Printf(" <red>-%s</>", filterLabel)
				} else if negate {
					orPass = true
					cout.Printf(" <green>-%s</>", filterLabel)
				} else {
					andFail = true
					cout.Printf(" <red>%s</>", filterLabel)
				}
			}
			cout.Println()

			if actionAnd {
				return !andFail, nil
			}

			return orPass, nil
		},
	}
}

func GetFilterForTitleRegex(pattern string) (*Filter, error) {
	if pattern == "" {
		return nil, nil
	}

	// Make the pattern case-insensitive by adding (?i) prefix
	caseInsensitivePattern := "(?i)" + pattern
	re, err := regexp.Compile(caseInsensitivePattern)
	if err != nil {
		return nil, fmt.Errorf("invalid title regex pattern '%s': %w", pattern, err)
	}

	cout.Printf("  title regex: <magenta>%s</> (case-insensitive)\n", pattern)

	return &Filter{
		Name: "title-regex",
		PR: func(pr github.PullRequest) (bool, error) {
			title := pr.GetTitle()

			if re.MatchString(title) {
				cout.Printf("    title: <green>%s</>\n", title)
				return true, nil
			}

			cout.Printf("    title: <red>%s</>\n", title)
			return false, nil
		},
	}, nil
}
