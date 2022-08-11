package cli

import (
	"fmt"
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

	branch := &cobra.Command{
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
	}
	root.AddCommand(branch)

	pr := &cobra.Command{
		Use:           "pr # [test_regex]",
		Short:         "triggers acceptance tests matching regex for a PR",
		Long:          `For a given PR number, discovers and runs acceptance tests against that PR branch.`,
		Args:          cobra.RangeArgs(1, 2),
		PreRunE:       ValidateParams([]string{"server", "buildtypeid", "repo", "fileregex", "splitteston"}),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			prs := args[0]
			testRegExParam := "TestAcc" // default to all tests

			if len(args) == 2 {
				testRegExParam = args[1]
			}

			// At this point command validation has been done so any more errors don't require help to be printed
			cmd.SilenceUsage = true

			f := GetFlags()

			// parse pr list
			numbers := []int{}
			for _, pr := range strings.Split(prs, ",") {
				pri, err := strconv.Atoi(pr)
				if err != nil {
					c.Printf("<red>ERROR:</> parsing PRs: unable to convert '%s' into an integer: %w\n", pr, err)
					continue
				}

				numbers = append(numbers, pri)
			}

			for _, pri := range numbers {
				// get
				// StartBuildForPrTests()

				//

				serviceTests, err := f.GetPrTests(pri)
				if err != nil {
					c.Printf("  <red>ERROR: discovering tests:</> %v\n\n", err)
					continue
				}

				if serviceTests == nil {
					c.Printf("  <red>ERROR: service tests in nil</>\n\n")
					continue
				}

				// trigger a build for each service
				for s, tests := range *serviceTests {
					serviceInfo := ""
					if s != "" {
						serviceInfo = "[<yellow>" + s + "</>]"
					}

					// genreatae test regex if we don't have it
					testRegEx := testRegExParam
					if testRegEx == "" {
						// if no testregex and no tests throw an error (-a is required for all)
						if len(tests) == 0 {
							c.Printf("  %s<red>ERROR:</> no tests found, use -a to run all tests\n", serviceInfo)
							continue
						}

						testRegEx = "(" + strings.Join(tests, "|") + ")"
					}

					// if all tests switch set regex to TestAcc
					if viper.GetBool("alltests") {
						testRegEx = "TestAcc"
					}

					// if we have a service put it on the end of the build type id
					buildTypeID := viper.GetString("buildtypeid")
					if s != "" {
						buildTypeID += "_" + strings.ToUpper(s)
					}

					branch := fmt.Sprintf("refs/pull/%d/merge", pri)

					if err := GetFlags().BuildCmd(buildTypeID, branch, testRegEx, serviceInfo); err != nil {
						c.Printf("  <red>ERROR: Unable to trigger build:</> %v\n", err)
					}
					fmt.Println()
				}
			}

			return nil
		},
	}
	root.AddCommand(pr)

	list := &cobra.Command{
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
	}
	root.AddCommand(list)

	results := &cobra.Command{
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

			return GetFlags().TestResultsCmd(buildID)
		},
	}
	root.AddCommand(results)

	resultsByPR := &cobra.Command{
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

			return GetFlags().TestResultsByPRCmd(pr)
		},
	}
	results.AddCommand(resultsByPR)

	if err := configureFlags(root); err != nil {
		return nil, fmt.Errorf("unable to configure flags: %w", err)
	}

	return root, nil
}
