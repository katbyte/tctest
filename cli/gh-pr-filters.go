package cli

import (
	"strings"

	"github.com/google/go-github/v45/github"
	c "github.com/gookit/color"
	//nolint:misspell
)

type Filter struct {
	Name   string
	PR     func(github.PullRequest) bool
	Output func(github.PullRequest)
}

func (f FlagData) GetFilters() []Filter {

	var filters []Filter

	if f := GetFilterForAuthors(f.GH.FilterPRs.Authors); f != nil {
		filters = append(filters, *f)
	}

	if f := GetFilterForLabelsOr(f.GH.FilterPRs.LabelsOr); f != nil {
		filters = append(filters, *f)
	}

	if f := GetFilterForLabelsAnd(f.GH.FilterPRs.LabelsAnd); f != nil {
		filters = append(filters, *f)
	}

	return filters
}

func GetFilterForAuthors(authors []string) *Filter {
	if len(authors) <= 0 {
		return nil
	}

	authorMap := map[string]bool{}
	for _, a := range authors {
		authorMap[a] = true
	}

	c.Printf("    authors: <magenta>%s</>\n", strings.Join(authors, "</>,<magenta>"))

	return &Filter{
		Name: "authors",
		PR: func(pr github.PullRequest) bool {
			author := pr.User.GetLogin()

			if _, ok := authorMap[author]; ok {
				c.Printf("    author: <green>%s</>\n", author)
				return true
			}

			return false
		},
		Output: nil,
	}
}

func GetFilterForLabelsOr(labels []string) *Filter {
	return GetFilterForLabels(labels, false)
}

func GetFilterForLabelsAnd(labels []string) *Filter {
	return GetFilterForLabels(labels, true)
}

func GetFilterForLabels(labels []string, and bool) *Filter {
	if len(labels) <= 0 {
		return nil
	}

	labelMap := map[string]bool{}
	for _, l := range labels {
		b := !strings.HasPrefix(l, "-")
		l = strings.TrimPrefix(l, "-")
		labelMap[l] = b
	}

	action := "or"
	if and {
		action = "ant"
	}

	c.Printf("  labels %s:  <blue>%s</>\n", strings.Join(labels, "</>,<blue>"), action)

	found := false
	return func(pr github.PullRequest) bool {
		for _, l := range pr.Labels {
			labels = append(labels, l.GetName())
			v, ok := labelMap[l.GetName()]
			if ok && v {
				return true
			}
		}

		return false
	}
}
