package cmd

import (
	"fmt"
	"strings"

	"github.com/katbyte/tctest/version"
	"github.com/spf13/cobra"
)

type TCFlags struct {
	ServerUrl   string
	BuildTypeId string
	User        string
	Pass        string
}

type PRFlags struct {
	Repo      string
	FileRegEx string
	TestSplit string
}

type Flags struct {
	TC TCFlags
	PR PRFlags
}

// colours
// PR - cyan
// urls dim blue
// tests - pink/purple
//

// OUTPUT
//discovering tests (github url to PR)
// test1 colour?)
// test2
//triggering build_id(white) @ BRANCH(white) with PATTERN(white)...
//  started build dim green) #123(bright green) (url to build) (dim)
//if wait, live update buildlog every x seconds

func Make() *cobra.Command {

	flags := Flags{}

	root := &cobra.Command{
		Use:   "tctest branch [test regex]",
		Short: "tctest is a small utility to trigger acceptance tests on teamcity",
		Long: `A small utility to trigger acceptance tests on teamcity. 
It can also pull the tests to run for a PR on github
Complete documentation is available at https://github.com/katbyte/tctest`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			branch := args[0]
			testRegEx := args[1]

			return TcCmd(flags.TC.ServerUrl, flags.TC.BuildTypeId, branch, testRegEx, flags.TC.User, flags.TC.Pass)
		},
	}

	pr := &cobra.Command{
		Use:   "pr # [test_regex]",
		Short: "triggers acceptance tests matching regex for a PR",
		Long:  `TODO`,
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			pr := args[0]
			testRegEx := ""
			if len(args) == 2 {
				testRegEx = args[1]
			}
			branch := fmt.Sprintf("refs/pull/%s/merge", pr)

			auto, err := cmd.Flags().GetBool("auto")
			if err != nil {
				return fmt.Errorf("failed to get auto state: %v", err)
			}

			if auto {
				tests, err := PrCmd(flags.PR.Repo, pr, flags.PR.FileRegEx, flags.PR.TestSplit)
				if err != nil {
					return fmt.Errorf("pr cmd failed: %v", err)
				}

				testRegEx = "(" + strings.Join(*tests, "|") + ")"

			}

			return TcCmd(flags.TC.ServerUrl, flags.TC.BuildTypeId, branch, testRegEx, flags.TC.User, flags.TC.Pass)
		},
	}
	root.AddCommand(pr)

	pr.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "attempts to discover what acceptance tests to run for a PR",
		Long:  `TODO`,
		Args:  cobra.RangeArgs(1, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pr := args[0]

			if _, err := PrCmd(flags.PR.Repo, pr, flags.PR.FileRegEx, flags.PR.TestSplit); err != nil {
				return fmt.Errorf("pr cmd failed: %v", err)
			}

			return nil
		},
	})

	pr.Flags().BoolP("auto", "a", false, "automatically discovery tests from PR files")

	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version number of tctest",
		Long:  `Print the version number of tctest`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("tctest v" + version.Version + "-" + version.GitCommit)
		},
	})

	root.PersistentFlags().StringVarP(&flags.PR.Repo, "repo", "r", "", "repository the pr resides in, such as `terraform-providers/terraform-provider-azurerm`")
	root.PersistentFlags().StringVarP(&flags.PR.FileRegEx, "fileregex", "", "(^[a-z]*/resource_|^[a-z]*/data_source_)", "the regex to filter files by`")
	root.PersistentFlags().StringVar(&flags.PR.TestSplit, "splittests", "_", "split tests here and use the value on the left")

	root.PersistentFlags().StringVarP(&flags.TC.ServerUrl, "server", "s", "", "the TeamCity server's ur;")
	root.PersistentFlags().StringVarP(&flags.TC.BuildTypeId, "buildtypeid", "b", "", "the TeamCity BuildTypeId to trigger")
	root.PersistentFlags().StringVarP(&flags.TC.User, "user", "u", "", "the TeamCity user to use")
	root.PersistentFlags().StringVarP(&flags.TC.Pass, "pass", "p", "", "the TeamCity password to use (unless you know what your doing don't use this!)")
	// todo viper for env files

	// todo validate we have enough info

	return root
}
