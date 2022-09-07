package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v45/github"
	c "github.com/gookit/color" //nolint:misspell
)

type Filter struct {
	Name string
	PR   func(github.PullRequest) (bool, error) // todo shjould this return an error?
}

func (f FlagData) GetFilters() []Filter {
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

	fmt.Println()

	return filters
}

func GetFilterForAuthors(authors []string) *Filter {
	if len(authors) == 0 {
		return nil
	}

	authorMap := map[string]bool{}
	for _, a := range authors {
		authorMap[a] = true
	}

	c.Printf("  authors: <magenta>%s</>\n", strings.Join(authors, "</>,<magenta>"))

	return &Filter{
		Name: "authors",
		PR: func(pr github.PullRequest) (bool, error) {
			author := pr.User.GetLogin()

			if _, ok := authorMap[author]; ok {
				c.Printf("    author: <green>%s</>\n", author)
				return true, nil
			}
			c.Printf("    author: <red>%s</>\n", author)

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

	c.Printf("  milestone: <magenta>%s</>\n", milestoneRaw)

	return &Filter{
		Name: "milestones",
		PR: func(pr github.PullRequest) (bool, error) {
			milestone := pr.GetMilestone().GetTitle()

			// nolint:gocritic
			if strings.EqualFold(filterMilestone, milestone) && !negate {
				c.Printf("    milestone: <green>%s</> <gray>(%s)</>\n", filterMilestone, milestone)
				return true, nil
			} else if negate {
				c.Printf("    milestone: <green>-%s</> <gray>(%s)</>\n", filterMilestone, milestone)
				return true, nil
			} else {
				c.Printf("    milestone: <red>%s</> <gray>(%s)</>\n", filterMilestone, milestone)
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

	c.Printf("  created within: <magenta>%s</>\n", duration.String())

	return &Filter{
		Name: "creation-time",
		PR: func(pr github.PullRequest) (bool, error) {
			createdAt := pr.GetCreatedAt()

			if createdAt.After(cutoffTime) {
				c.Printf("    created: <green>%s</> <gray>(%s)</>\n", createdAt.Format(time.RFC822), cutoffTime.Format(time.RFC822))
				return true, nil
			}

			c.Printf("    created: <red>%s</> <gray>(%s)</>\n", createdAt.Format(time.RFC822), cutoffTime.Format(time.RFC822))

			return false, nil
		},
	}
}

func GetFilterForUpdatedTime(duration time.Duration) *Filter {
	if duration == time.Nanosecond {
		return nil
	}
	cutoffTime := time.Now().Add(-duration)

	c.Printf("  updated within: <magenta>%s</>\n", duration.String())

	return &Filter{
		Name: "creation-time",
		PR: func(pr github.PullRequest) (bool, error) {
			createdAt := pr.GetUpdatedAt()

			if createdAt.After(cutoffTime) {
				c.Printf("    updated: <green>%s</> <gray>(%s)</>\n", createdAt.Format(time.RFC822), cutoffTime.Format(time.RFC822))
				return true, nil
			}

			c.Printf("    updated: <red>%s</> <gray>(%s)</>\n", createdAt.Format(time.RFC822), cutoffTime.Format(time.RFC822))

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

	c.Printf("  labels %s:  <blue>%s</>\n", action, strings.Join(labels, "</>,<blue>"))

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
				c.Printf("    labels all: ")
			} else {
				c.Printf("    labels any: ")
			}

			andFail := false
			orPass := false

			// for each filter label see if it exists
			for filterLabel, negate := range filterLabelMap {
				_, found := labelMap[filterLabel]

				// nolint:gocritic
				if found && !negate {
					orPass = true
					c.Printf(" <green>%s</>", filterLabel)
				} else if found && negate {
					andFail = true
					c.Printf(" <red>%s</>", filterLabel)
				} else if negate {
					orPass = true
					c.Printf(" <green>-%s</>", filterLabel)
				} else {
					andFail = true
					c.Printf(" <red>%s</>", filterLabel)
				}
			}
			fmt.Println()

			if actionAnd {
				return !andFail, nil
			}

			return orPass, nil
		},
	}
}
