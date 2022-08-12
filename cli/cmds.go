package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/katbyte/tctest/lib/version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	//nolint:misspell
	c "github.com/gookit/color"
)

func ValidateParams(params []string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		for _, p := range params {
			if viper.GetString(p) == "" {
				return fmt.Errorf(p + " parameter can't be empty")
			}
		}

		return nil
	}
}

func Make() (*cobra.Command, error) {
	// This is a no-op to avoid accidentally triggering broken builds on malformed commands
	root := &cobra.Command{
		Use:   "tctest [command]",
		Short: "tctest is a small utility to trigger acceptance tests on teamcity",
		Long: `A small utility to trigger acceptance tests on teamcity. 
It can also pull the tests to run for a PR on github
Complete documentation is available at https://github.com/katbyte/tctest`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Run \"tctest help\" for more information about available tctest commands.\n")
			return nil
		},
	}

	root.AddCommand(&cobra.Command{
		Use:           "version",
		Short:         "Print the version number of tctest",
		Long:          `Print the version number of tctest`,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("tctest v" + version.Version + "-" + version.GitCommit)
		},
	})

	root.AddCommand(&cobra.Command{
		Use:           "branch [branchName] [test regex]",
		Short:         "triggers acceptance tests matching regex for a branch name",
		Long:          `For a given branch name and regex, discovers and runs acceptance tests against that branch.`,
		Aliases:       []string{"b"},
		Args:          cobra.ExactArgs(2),
		PreRunE:       ValidateParams([]string{"server", "buildtypeid"}),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			branch := args[0]
			testRegEx := args[1]

			if !strings.HasPrefix(branch, "refs/") {
				branch = "refs/heads/" + branch
			}

			// At this point command validation has been done so any more errors don't require help to be printed
			cmd.SilenceUsage = true
			f := GetFlags()

			return f.BuildCmd(f.TC.Build.TypeID, branch, testRegEx, "")
		},
	})

	root.AddCommand(&cobra.Command{
		Use:           "pr # [test_regex]",
		Short:         "triggers acceptance tests matching regex for a PR",
		Long:          `For a given PR number, discovers and runs acceptance tests against that PR branch.`,
		Args:          cobra.RangeArgs(1, 2),
		PreRunE:       ValidateParams([]string{"server", "buildtypeid", "repo", "fileregex", "splitteston"}),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			prs := args[0]
			testRegExParam := ""

			if len(args) == 2 {
				testRegExParam = args[1]
			}

			// At this point command validation has been done so any more errors don't require help to be printed
			cmd.SilenceUsage = true

			// parse list of prs
			numbers := []int{}
			for _, pr := range strings.Split(prs, ",") {
				pri, err := strconv.Atoi(pr)
				if err != nil {
					c.Printf("<red>ERROR:</> parsing PRs: unable to convert '%s' into an integer: %v\n", pr, err)
					continue
				}

				numbers = append(numbers, pri)
			}

			return GetFlags().GetAndRunPrsTests(numbers, testRegExParam)
		},
	})

	root.AddCommand(&cobra.Command{
		Use:           "prs [test_regex] [-a author1,katbyte] [-l with-this-label,-not-this-label]",
		Short:         "triggers acceptance tests for each open PR matching specified filters",
		Long:          `TODO.`,
		Args:          cobra.RangeArgs(0, 1),
		PreRunE:       ValidateParams([]string{"server", "buildtypeid", "repo", "fileregex", "splitteston"}),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			testRegExParam := ""
			if len(args) == 1 {
				testRegExParam = args[0]
			}

			// At this point command validation has been done so any more errors don't require help to be printed
			cmd.SilenceUsage = true
			f := GetFlags()
			r := f.NewRepo()

			// get all open PRs
			// get all pull requests
			c.Printf("Retrieving all prs for <white>%s</>/<cyan>%s</>...", r.Owner, r.Name)
			prs, err := r.GetAllPullRequests("open")
			if err != nil {
				c.Printf("\n\n <red>ERROR!!</> %s\n", err)
				return nil
			}
			c.Printf(" found <yellow>%d</>\n", len(*prs))

			// process filters
			filterAuthors := len(f.GH.FilterPRs.Authors) != 0
			filterLabels := len(f.GH.FilterPRs.Labels) != 0

			authorMap := map[string]bool{}
			if filterAuthors {
				for _, a := range f.GH.FilterPRs.Authors {
					authorMap[a] = true
				}
				c.Printf("  authors: <magenta>%s</>\n", strings.Join(f.GH.FilterPRs.Authors, "</>,<magenta>"))
			}

			labelMap := map[string]bool{}
			if filterLabels {
				for _, l := range f.GH.FilterPRs.Labels {
					b := !strings.HasPrefix(l, "-")
					l = strings.TrimPrefix(l, "-")
					labelMap[l] = b
				}
				c.Printf("  labels:  <blue>%s</>\n", strings.Join(f.GH.FilterPRs.Labels, "</>,<blue>"))
			}

			var numbers []int
			for _, pr := range *prs {
				test := false
				user := pr.User.GetLogin()
				number := pr.GetNumber()
				name := pr.GetTitle()

				// if no filters we test
				if !filterAuthors && !filterLabels {
					test = true
				}

				if filterAuthors {
					if _, ok := authorMap[user]; filterAuthors && ok {
						test = true
					}
				}

				labels := []string{}
				for _, l := range pr.Labels {
					labels = append(labels, l.GetName())
				}

				if filterLabels {
					found := false
					for _, l := range pr.Labels {
						labels = append(labels, l.GetName())
						v, ok := labelMap[l.GetName()]
						if ok && v {
							found = true
						}
					}

					test = test && found
				}

				if test {
					// todo highlight labels matched
					c.Printf(" #<green>%d</> <magenta>%s</> %s - <white>%s</> \n", number, user, strings.Join(labels, "<white>,</>"), name)
					numbers = append(numbers, number)
				} else {
					// todo log
					// c.Printf(" #<red>%d</> <magenta>%s</> %s - <white>%s</> \n", number, user, strings.Join(labels, "<white>,</>"), name)
				}
			}

			sort.Ints(numbers)
			return GetFlags().GetAndRunPrsTests(numbers, testRegExParam)
		},
	})

	root.AddCommand(&cobra.Command{
		Use:           "list #",
		Short:         "attempts to discover what acceptance tests to run for a PR",
		Long:          `For a given PR number, attempts to discover and list what acceptance tests would run for it, without actually triggering a build.`,
		Args:          cobra.RangeArgs(1, 1),
		PreRunE:       ValidateParams([]string{"repo", "fileregex", "splitteston"}),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			pr, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("pr should be a number: %w", err)
			}

			cmd.SilenceUsage = true

			_, err = GetFlags().GetPrTests(pr)

			return err
		},
	})

	root.AddCommand(&cobra.Command{
		Use:           "results #",
		Short:         "shows the test results for a specified TC build ID",
		Long:          "Shows the test results for a specified TC build ID. If the build is still in progress, it will warn the user that results may be incomplete.",
		Args:          cobra.RangeArgs(1, 1),
		PreRunE:       ValidateParams([]string{"server"}),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			buildID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("pr should be a number: %w", err)
			}

			cmd.SilenceUsage = true

			return GetFlags().BuildResultsCmd(buildID)
		},
	})

	root.AddCommand(&cobra.Command{
		Use:           "pr #",
		Short:         "shows the test results for a specified PR #",
		Long:          "Shows the test results for a specified PR #. If the build is still in progress, it will warn the user that results may be incomplete.",
		Args:          cobra.RangeArgs(1, 1),
		PreRunE:       ValidateParams([]string{"server", "buildtypeid"}),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			pr, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("pr should be a number: %w", err)
			}

			cmd.SilenceUsage = true

			return GetFlags().BuildResultsForPRCmd(pr)
		},
	})

	if err := configureFlags(root); err != nil {
		return nil, fmt.Errorf("unable to configure flags: %w", err)
	}

	return root, nil
}
