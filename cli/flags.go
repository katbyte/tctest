package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type FlagData struct {
	GH            FlagsGitHub
	TC            FlagsTeamCity
	OpenInBrowser bool
	RunAllTests   bool
}

type FlagsGitHub struct {
	Token        string
	Repo         string
	FileRegEx    string
	SplitTestsOn string
	FilterPRs    FlagsGitHubPrFilter
}

type FlagsGitHubPrFilter struct {
	Authors      []string
	LabelsOr     []string
	LabelsAnd    []string
	Milestone    string
	TitleRegex   string
	Drafts       bool
	CreationTime time.Duration
	UpdatedTime  time.Duration
}

type FlagsTeamCity struct {
	Build     FlagsTeamCityBuild
	ServerURL string
	Token     string
	User      string
	Pass      string
}

type FlagsTeamCityBuild struct {
	TypeID       string
	Parameters   string
	SkipQueue    bool
	Wait         bool
	Latest       bool
	QueueTimeout int
	RunTimeout   int
	Tags         []string
}

func configureFlags(root *cobra.Command) error {
	flags := FlagData{}
	pflags := root.PersistentFlags()

	pflags.BoolVarP(&flags.OpenInBrowser, "open", "o", false, "Open the PR and build in a browser")
	pflags.BoolVarP(&flags.RunAllTests, "all", "", false, "run all tests when none are found by passing TestAcc")

	pflags.StringVar(&flags.GH.Token, "token-gh", "", "github oauth token (consider exporting token to GITHUB_TOKEN instead)")
	pflags.StringVarP(&flags.GH.Repo, "repo", "r", "", "repository the pr resides in, such as terraform-providers/terraform-provider-azurerm")
	pflags.StringVar(&flags.GH.FileRegEx, "fileregex", "(^[a-z]*/resource_|^[a-z]*/data_source_)", "the regex to filter files by`")
	pflags.StringVar(&flags.GH.SplitTestsOn, "splitteston", "_", "the character to split tests on and use the value on the left")

	pflags.StringSliceVarP(&flags.GH.FilterPRs.Authors, "f-authors", "a", []string{}, "only test PR by these authors. ie 'katbyte,author2,author3'")
	pflags.StringSliceVarP(&flags.GH.FilterPRs.LabelsAnd, "f-labels-all", "l", []string{}, "only test PRs that match all label conditions. ie 'label1,label2,-not-this-label'")
	pflags.StringSliceVarP(&flags.GH.FilterPRs.LabelsOr, "f-labels-any", "", []string{}, "only test PRs that match any label conditions. ie 'label1,label2,-not-this-label'")
	pflags.StringVarP(&flags.GH.FilterPRs.Milestone, "f-milestone", "m", "", "filter out PRs that have or do no have a milestone, ie 'this-milstone' or '-not-this-milestone'")
	pflags.DurationVarP(&flags.GH.FilterPRs.CreationTime, "f-created-time", "", time.Nanosecond, "filter out PRs that where not created within this duration")
	pflags.DurationVarP(&flags.GH.FilterPRs.UpdatedTime, "f-updated-time", "", time.Nanosecond, "filter out PRs that where not created within this duration")
	pflags.StringVarP(&flags.GH.FilterPRs.TitleRegex, "f-title-regex", "", "", "filter PRs by title using case-insensitive regex (e.g. 'test' matches titles containing 'test', 'fix.*bug' matches 'fix' followed by 'bug')")
	pflags.BoolVarP(&flags.GH.FilterPRs.Drafts, "f-drafts", "d", false, "filter out any PRs that are in draft more")

	pflags.StringVarP(&flags.TC.ServerURL, "server", "s", "", "the TeamCity server's url")
	pflags.StringVarP(&flags.TC.Token, "token-tc", "t", "", "the TeamCity token to use (consider exporting token to TCTEST_TOKEN_TC instead)")
	pflags.StringVar(&flags.TC.User, "username", "", "the TeamCity user to use")
	pflags.StringVar(&flags.TC.Pass, "password", "", "the TeamCity password to use (consider exporting pass to TCTEST_PASS instead)")
	pflags.StringVarP(&flags.TC.Build.TypeID, "buildtypeid", "b", "", "the TeamCity BuildTypeId to trigger")
	pflags.StringVarP(&flags.TC.Build.Parameters, "properties", "p", "", "the TeamCity build parameters to use in 'KEY1=VALUE1;KEY2=VALUE2' format")
	pflags.BoolVarP(&flags.TC.Build.SkipQueue, "skip-queue", "q", false, "Put the build to the queue top")
	pflags.BoolVarP(&flags.TC.Build.Wait, "wait", "w", false, "Wait for the build to complete before tctest exits")
	pflags.BoolVarP(&flags.TC.Build.Latest, "latest", "", false, "gets the latest build in TeamCity")
	pflags.IntVarP(&flags.TC.Build.QueueTimeout, "queue-timeout", "", 60, "How long to wait for a queued build to start running before tctest times out")
	pflags.IntVarP(&flags.TC.Build.RunTimeout, "run-timeout", "", 60, "How long to wait for a running build to finish before tctest times out")
	pflags.StringSliceVarP(&flags.TC.Build.Tags, "tag", "", []string{}, "TeamCity build tags to add to the triggered build, ie 'tag1,tag2'")

	// binding map for viper/pflag -> env
	m := map[string]string{
		"server":         "TCTEST_SERVER",
		"buildtypeid":    "TCTEST_BUILDTYPEID",
		"token-tc":       "TCTEST_TOKEN_TC",
		"token-gh":       "GITHUB_TOKEN",
		"username":       "TCTEST_USER",
		"password":       "TCTEST_PASS",
		"properties":     "TCTEST_PROPERTIES",
		"repo":           "TCTEST_REPO",
		"fileregex":      "TCTEST_FILEREGEX",
		"splitteston":    "TCTEST_SPLIT_TESTS_ON",
		"wait":           "TCTEST_WAIT",
		"all":            "",
		"queue-timeout":  "",
		"run-timeout":    "",
		"f-authors":      "",
		"f-milestone":    "",
		"f-labels-all":   "",
		"f-labels-any":   "",
		"f-created-time": "",
		"f-updated-time": "",
		"f-title-regex":  "",
		"latest":         "TCTEST_LATESTBUILD",
		"skip-queue":     "TCTEST_SKIP_QUEUE",
		"open":           "TCTEST_OPEN_BROWSER",
		"tag":            "TCTEST_BUILD_TAGS",
	}

	for name, env := range m {
		if err := viper.BindPFlag(name, pflags.Lookup(name)); err != nil {
			return fmt.Errorf("error binding '%s' flag: %w", name, err)
		}

		if env != "" {
			if err := viper.BindEnv(name, env); err != nil {
				return fmt.Errorf("error binding '%s' to env '%s' : %w", name, env, err)
			}
		}
	}

	// todo config file
	/*viper.SetConfigName("config") // name of config file (without extension)
	viper.AddConfigPath("/etc/appname/")   // path to look for the config file in
	viper.AddConfigPath("$HOME/.appname")  // call multiple times to add many search paths
	viper.AddConfigPath(".")               // optionally look for config in the working directory
	err := viper.ReadInConfig() // Find and read the config file
	if err != nil { // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}*/

	return nil
}

func GetFlags() FlagData {
	// there has to be an easier way....
	return FlagData{
		OpenInBrowser: viper.GetBool("open"),
		RunAllTests:   viper.GetBool("all"),
		GH: FlagsGitHub{
			Repo:         viper.GetString("repo"),
			Token:        viper.GetString("token-gh"),
			FileRegEx:    viper.GetString("fileregex"),
			SplitTestsOn: viper.GetString("splitteston"),
			FilterPRs: FlagsGitHubPrFilter{
				Authors:      viper.GetStringSlice("f-authors"),
				LabelsOr:     viper.GetStringSlice("f-labels-any"),
				LabelsAnd:    viper.GetStringSlice("f-labels-all"),
				Milestone:    viper.GetString("f-milestone"),
				TitleRegex:   viper.GetString("f-title-regex"),
				CreationTime: viper.GetDuration("f-created-time"),
				UpdatedTime:  viper.GetDuration("f-updated-time"),
				Drafts:       viper.GetBool("f-draft"),
			},
		},
		TC: FlagsTeamCity{
			ServerURL: viper.GetString("server"),
			Token:     viper.GetString("token-tc"),
			User:      viper.GetString("username"),
			Pass:      viper.GetString("password"),
			Build: FlagsTeamCityBuild{
				TypeID:       viper.GetString("buildtypeid"),
				Parameters:   viper.GetString("properties"),
				SkipQueue:    viper.GetBool("skip-queue"),
				Wait:         viper.GetBool("wait"),
				Latest:       viper.GetBool("wait"),
				QueueTimeout: viper.GetInt("queue-timeout"),
				RunTimeout:   viper.GetInt("run-timeout"),
				Tags:         viper.GetStringSlice("tag"),
			},
		},
	}
}
