package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/katbyte/tctest/version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type TCFlags struct {
	ServerURL   string
	BuildTypeID string
	Token       string
	Parameters  string
}

type PRFlags struct {
	Repo      string
	FileRegEx string
	TestSplit string
}

type WaitFlags struct {
	Wait         bool
	QueueTimeout int
	RunTimeout   int
}

type FlagData struct {
	TC                  TCFlags
	PR                  PRFlags
	Wait                WaitFlags
	ServicePackagesMode bool
}

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

func Make() *cobra.Command {
	flags := FlagData{}

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

			server := viper.GetString("server")
			token := viper.GetString("token")
			buildTypeId := viper.GetString("buildtypeid")
			password := viper.GetString("password")
			properties := viper.GetString("properties")
			username := viper.GetString("username")
			wait := viper.GetBool("wait")

			if token != "" {
				tc := NewTeamCityUsingTokenAuth(server, token)
				return tc.Command(buildTypeId, properties, branch, testRegEx, wait)
			}

			tc := NewTeamCityUsingBasicAuth(server, username, password)
			return tc.Command(buildTypeId, properties, branch, testRegEx, wait)
		},
	}
	root.AddCommand(branch)

	pr := &cobra.Command{
		Use:           "pr # [test_regex]",
		Short:         "triggers acceptance tests matching regex for a PR",
		Long:          `For a given PR number, discovers and runs acceptance tests against that PR branch.`,
		Args:          cobra.RangeArgs(1, 2),
		PreRunE:       ValidateParams([]string{"server", "buildtypeid", "repo", "fileregex", "splittests"}),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			pr := args[0]
			testRegEx := ""
			repo := viper.GetString("repo")

			if len(args) == 2 {
				testRegEx = args[1]
			}
			if _, err := strconv.Atoi(pr); err != nil {
				return fmt.Errorf("pr should be a number: %v", err)
			}

			cmd.SilenceUsage = true

			state, err := PrState(repo, pr)
			if err != nil {
				return fmt.Errorf("unable to get pr state: %v", err)
			}
			if state == "closed" {
				return fmt.Errorf("cannot start build for a closed pr")
			}

			if testRegEx == "" {
				tests, err := PrCmd(viper.GetString("repo"), pr, viper.GetString("fileregex"), viper.GetString("splittests"), viper.GetBool("servicepackages"))
				if err != nil {
					return fmt.Errorf("pr cmd failed: %v", err)
				}

				testRegEx = "(" + strings.Join(*tests, "|") + ")"
			}

			server := viper.GetString("server")
			buildTypeId := viper.GetString("buildtypeid")
			branch := fmt.Sprintf("refs/pull/%s/merge", pr)
			token := viper.GetString("token")
			properties := viper.GetString("properties")
			password := viper.GetString("password")
			username := viper.GetString("username")
			wait := viper.GetBool("wait")

			if token != "" {
				tc := NewTeamCityUsingTokenAuth(server, token)
				return tc.Command(buildTypeId, properties, branch, testRegEx, wait)
			}

			tc := NewTeamCityUsingBasicAuth(server, username, password)
			return tc.Command(buildTypeId, properties, branch, testRegEx, wait)
		},
	}
	root.AddCommand(pr)

	list := &cobra.Command{
		Use:           "list #",
		Short:         "attempts to discover what acceptance tests to run for a PR",
		Long:          `For a given PR number, attempts to discover and list what acceptance tests would run for it, without actually triggering a build.`,
		Args:          cobra.RangeArgs(1, 1),
		PreRunE:       ValidateParams([]string{"repo", "fileregex", "splittests"}),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			pr := args[0]

			cmd.SilenceUsage = true

			if _, err := PrCmd(viper.GetString("repo"), pr, viper.GetString("fileregex"), viper.GetString("splittests"), viper.GetBool("servicepackages")); err != nil {
				return fmt.Errorf("pr cmd failed: %v", err)
			}
			return nil
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
			buildId := args[0]

			cmd.SilenceUsage = true

			server := viper.GetString("server")
			password := viper.GetString("password")
			username := viper.GetString("username")
			token := viper.GetString("token")
			wait := viper.GetBool("wait")

			if token != "" {
				tc := NewTeamCityUsingTokenAuth(server, token)
				return tc.testResults(buildId, wait)
			}

			tc := NewTeamCityUsingBasicAuth(server, username, password)
			return tc.testResults(buildId, wait)
		},
	}
	root.AddCommand(results)

	pflags := root.PersistentFlags()
	pflags.StringVarP(&flags.TC.ServerURL, "server", "s", "", "the TeamCity server's url")
	pflags.StringVarP(&flags.TC.BuildTypeID, "buildtypeid", "b", "", "the TeamCity BuildTypeId to trigger")
	pflags.StringVarP(&flags.TC.Token, "token", "k", "", "the TeamCity token to use (consider exporting pass to TCTEST_TOKEN instead)")
	pflags.StringVarP(&flags.TC.User, "user", "u", "", "the TeamCity user to use")
	pflags.StringVarP(&flags.TC.Pass, "pass", "p", "", "the TeamCity password to use (consider exporting pass to TCTEST_PASS instead)")
	pflags.StringVarP(&flags.TC.Parameters, "properties", "", "", "the TeamCity build parameters to use in 'KEY1=VALUE1;KEY2=VALUE2' format")

	pflags.StringVarP(&flags.PR.Repo, "repo", "r", "", "repository the pr resides in, such as terraform-providers/terraform-provider-azurerm")
	pflags.StringVarP(&flags.PR.FileRegEx, "fileregex", "", "(^[a-z]*/resource_|^[a-z]*/data_source_)", "the regex to filter files by`")
	pflags.StringVar(&flags.PR.TestSplit, "splittests", "_", "split tests here and use the value on the left")

	pflags.BoolVar(&flags.ServicePackagesMode, "servicepackages", false, "enable service packages mode for AzureRM")

	pflags.BoolVarP(&flags.Wait.Wait, "wait", "w", false, "Wait for the build to complete before tctest exits")
	pflags.IntVarP(&flags.Wait.QueueTimeout, "queue-timeout", "q", 60, "How long to wait for a queued build to start running before tctest times out")
	pflags.IntVarP(&flags.Wait.RunTimeout, "run-timeout", "t", 60, "How long to wait for a running build to finish before tctest times out")

	// binding map for viper/pflag -> env
	m := map[string]string{
		"server":          "TCTEST_SERVER",
		"buildtypeid":     "TCTEST_BUILDTYPEID",
		"token":           "TCTEST_TOKEN",
		"username":        "TCTEST_USER",
		"password":        "TCTEST_PASS",
		"properties":      "TCTEST_PROPERTIES",
		"repo":            "TCTEST_REPO",
		"fileregex":       "TCTEST_FILEREGEX",
		"splittests":      "TCTEST_SPLITTESTS",
		"servicepackages": "TCTEST_SERVICEPACKAGESMODE",
		"wait":            "TCTEST_WAIT",
		"queue-timeout":   "",
		"run-timeout":     "",
	}

	for name, env := range m {
		if err := viper.BindPFlag(name, pflags.Lookup(name)); err != nil {
			fmt.Println(fmt.Errorf("error binding '%s' flag: %v", name, err))
		}

		if env != "" {
			if err := viper.BindEnv(name, env); err != nil {
				fmt.Println(fmt.Errorf("error binding '%s' to env '%s' : %v", name, env, err))
			}
		}
	}

	//todo config file
	/*viper.SetConfigName("config") // name of config file (without extension)
	viper.AddConfigPath("/etc/appname/")   // path to look for the config file in
	viper.AddConfigPath("$HOME/.appname")  // call multiple times to add many search paths
	viper.AddConfigPath(".")               // optionally look for config in the working directory
	err := viper.ReadInConfig() // Find and read the config file
	if err != nil { // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}*/

	return root
}
