package cli

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/katbyte/tctest/lib/clog"
	"github.com/katbyte/tctest/lib/tc"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// resolveBuildTypeID handles the legacy --buildtypeid to --build-type-id migration.
// It errors if both are set. When only the old flag is used, it copies the value to
// build-type-id and enables build-type-id-add-service-suffix to maintain the old behaviour.
// Called from PersistentPreRunE before ValidateParams so the resolved value is available for validation.
// TODO remove this at some point in the future.
func resolveBuildTypeID(cmd *cobra.Command) error {
	oldFlagSet := cmd.Flags().Changed("buildtypeid")
	newFlagSet := cmd.Flags().Changed("build-type-id")

	// error only when both CLI flags are explicitly provided
	if oldFlagSet && newFlagSet {
		return errors.New("cannot use both --buildtypeid and --build-type-id; --buildtypeid is deprecated, use --build-type-id only")
	}

	// explicit --buildtypeid CLI flag: copy to build-type-id and enable service suffix
	if oldFlagSet && !newFlagSet {
		viper.Set("build-type-id", viper.GetString("buildtypeid"))
		if !viper.GetBool("build-type-id-add-service-suffix") {
			viper.Set("build-type-id-add-service-suffix", true)
		}
		fmt.Fprintf(os.Stderr, "WARNING: --buildtypeid/-b is deprecated and will be removed in a future version.\n")
		fmt.Fprintf(os.Stderr, "  Use --build-type-id instead. Note: --buildtypeid automatically appends _SERVICE\n")
		fmt.Fprintf(os.Stderr, "  to the build type ID. To keep this behaviour, use --build-type-id-add-service-suffix.\n")
		return nil
	}

	// no explicit CLI flags: fall back to env vars
	if viper.GetString("build-type-id") == "" && viper.GetString("buildtypeid") != "" {
		viper.Set("build-type-id", viper.GetString("buildtypeid"))
		if !viper.GetBool("build-type-id-add-service-suffix") {
			viper.Set("build-type-id-add-service-suffix", true)
		}
	}

	return nil
}

type FlagData struct {
	GH              FlagsGitHub     `mapstructure:",squash"`
	TC              FlagsTeamCity   `mapstructure:",squash"`
	DiscoveryConfig DiscoveryConfig `mapstructure:",squash"`
	OpenInBrowser   bool            `mapstructure:"open"`
	RunAllTests     bool            `mapstructure:"all"`
	Services        []string        `mapstructure:"service"`
	Quiet           bool            `mapstructure:"quiet"`
	JSON            bool            `mapstructure:"json"`
	Silent          bool            `mapstructure:"silent"`
	DryRun          bool            `mapstructure:"dry-run"`
	Verbose         bool            `mapstructure:"verbose"`
}

type DiscoveryConfig struct {
	FileRegEx                *regexp.Regexp   `mapstructure:"-"`
	SplitTestsOn             string           `mapstructure:"splitteston"`
	ReappendSplitCharacter   bool             `mapstructure:"reappend-split-character"`
	AccTestFileSuffixRegexes []*regexp.Regexp `mapstructure:"-"`
	Concurrency              int              `mapstructure:"concurrency"`
	CollapseFilesAfter       int              `mapstructure:"collapse-files-after"`
}

type FlagsGitHub struct {
	Token     string              `mapstructure:"token-gh"`
	Repo      string              `mapstructure:"repo"`
	FilterPRs FlagsGitHubPrFilter `mapstructure:",squash"`
}

type FlagsGitHubPrFilter struct {
	Authors      []string      `mapstructure:"f-authors"`
	LabelsOr     []string      `mapstructure:"f-labels-any"`
	LabelsAnd    []string      `mapstructure:"f-labels-all"`
	Milestone    string        `mapstructure:"f-milestone"`
	TitleRegex   string        `mapstructure:"f-title-regex"`
	Drafts       bool          `mapstructure:"f-drafts"`
	CreationTime time.Duration `mapstructure:"f-created-time"`
	UpdatedTime  time.Duration `mapstructure:"f-updated-time"`
}

type FlagsTeamCity struct {
	Build     FlagsTeamCityBuild `mapstructure:",squash"`
	ServerURL string             `mapstructure:"server"`
	Token     string             `mapstructure:"token-tc"`
	User      string             `mapstructure:"username"`
	Pass      string             `mapstructure:"password"`
}

type FlagsTeamCityBuild struct {
	TypeID           string   `mapstructure:"build-type-id"`
	LegacyTypeID     string   `mapstructure:"buildtypeid"`
	Parameters       string   `mapstructure:"properties"`
	SkipQueue        bool     `mapstructure:"skip-queue"`
	Wait             bool     `mapstructure:"wait"`
	Latest           bool     `mapstructure:"latest"`
	Comment          bool     `mapstructure:"comment"`
	ForceOldUI       bool     `mapstructure:"build-link-force-old-ui"`
	AddServiceSuffix bool     `mapstructure:"build-type-id-add-service-suffix"`
	QueueTimeout     int      `mapstructure:"queue-timeout"`
	RunTimeout       int      `mapstructure:"run-timeout"`
	MaxBuildsPerPR   int      `mapstructure:"max-builds-per-pr"`
	Tags             []string `mapstructure:"tag"`
}

func configureFlags(root *cobra.Command) error {
	flags := FlagData{}
	pflags := root.PersistentFlags()

	pflags.BoolVarP(&flags.OpenInBrowser, "open", "o", false, "Open the PR and build in a browser")
	pflags.BoolVarP(&flags.RunAllTests, "all", "", false, "run all tests when none are found by passing TestAcc")
	pflags.StringSliceVar(&flags.Services, "service", []string{}, "force trigger builds for specific services (comma-separated), use 'all' to trigger all services")
	pflags.BoolVar(&flags.Quiet, "quiet", false, "minimal machine-readable output (pr@service@build url)")
	pflags.BoolVar(&flags.JSON, "json", false, "output build results as JSON array")
	pflags.BoolVar(&flags.Silent, "silent", false, "suppress all output")
	pflags.BoolVar(&flags.DryRun, "dry-run", false, "show what builds would be triggered without actually triggering them")
	pflags.BoolVarP(&flags.Verbose, "verbose", "v", false, "show detailed file listings and trace output")

	// "services?" matches both provider layouts: AWS(`service`) and Azure(`services`).
	pflags.String("fileregex", `^internal/services?/[^/]+/[a-z0-9_][^/]*$`, "the regex to filter files by")
	pflags.StringVar(&flags.DiscoveryConfig.SplitTestsOn, "splitteston", "_", "the character to split tests on and use the value on the left")
	pflags.StringSlice("acctest-file-suffix-regexes", []string{
		`^_resource.*_test$`,   // Azure, this will also covers test files like `linux_virtual_machine_scale_set_resource_auth_test.go`
		`^_test$`,              // both providers
		`^_list_test$`,         // AWS list data-source tests
		`^_identity_gen_test$`, // AWS generated identity tests
		`^_tags_gen_test$`,     // AWS generated tags tests
		`^_data_source_test$`,  // data-source tests (both providers)
	}, "comma-separated list of regex patterns to match acceptance test filenames suffix (without '.go')")
	pflags.BoolVar(&flags.DiscoveryConfig.ReappendSplitCharacter, "reappend-split-character", false, "whether to append the split character to the resulting test filter for more precise filtering")
	pflags.IntVar(&flags.DiscoveryConfig.Concurrency, "concurrency", 5, "maximum number of concurrent file downloads during test discovery")
	pflags.IntVar(&flags.DiscoveryConfig.CollapseFilesAfter, "collapse-files-after", 20, "collapse file listings to a count when there are more than this many files (0 to always show)")

	pflags.StringVar(&flags.GH.Token, "token-gh", "", "github oauth token (consider exporting token to GITHUB_TOKEN instead)")
	pflags.StringVarP(&flags.GH.Repo, "repo", "r", "", "repository the pr resides in, such as terraform-providers/terraform-provider-azurerm")

	pflags.StringSliceVarP(&flags.GH.FilterPRs.Authors, "f-authors", "a", []string{}, "only test PR by these authors. ie 'katbyte,author2,author3'")
	pflags.StringSliceVarP(&flags.GH.FilterPRs.LabelsAnd, "f-labels-all", "l", []string{}, "only test PRs that match all label conditions. ie 'label1,label2,-not-this-label'")
	pflags.StringSliceVarP(&flags.GH.FilterPRs.LabelsOr, "f-labels-any", "", []string{}, "only test PRs that match any label conditions. ie 'label1,label2,-not-this-label'")
	pflags.StringVarP(&flags.GH.FilterPRs.Milestone, "f-milestone", "m", "", "filter out PRs that have or do no have a milestone, ie 'this-milstone' or '-not-this-milestone'")
	pflags.DurationVarP(&flags.GH.FilterPRs.CreationTime, "f-created-time", "", time.Nanosecond, "filter out PRs that where not created within this duration")
	pflags.DurationVarP(&flags.GH.FilterPRs.UpdatedTime, "f-updated-time", "", time.Nanosecond, "filter out PRs that were not updated within this duration")
	pflags.StringVarP(&flags.GH.FilterPRs.TitleRegex, "f-title-regex", "", "", "filter PRs by title using case-insensitive regex (e.g. 'test' matches titles containing 'test', 'fix.*bug' matches 'fix' followed by 'bug')")
	pflags.BoolVarP(&flags.GH.FilterPRs.Drafts, "f-drafts", "d", false, "filter out any PRs that are in draft mode")

	pflags.StringVarP(&flags.TC.ServerURL, "server", "s", "", "the TeamCity server's url")
	pflags.StringVarP(&flags.TC.Token, "token-tc", "t", "", "the TeamCity token to use (consider exporting token to TCTEST_TOKEN_TC instead)")
	pflags.StringVar(&flags.TC.User, "username", "", "the TeamCity user to use")
	pflags.StringVar(&flags.TC.Pass, "password", "", "the TeamCity password to use (consider exporting pass to TCTEST_PASS instead)")
	pflags.StringVarP(&flags.TC.Build.LegacyTypeID, "buildtypeid", "b", "", "[DEPRECATED] use --build-type-id instead")
	pflags.StringVar(&flags.TC.Build.TypeID, "build-type-id", "", "the TeamCity BuildTypeId to trigger")
	pflags.BoolVar(&flags.TC.Build.AddServiceSuffix, "build-type-id-add-service-suffix", false, "append _SERVICE to the build type ID (legacy behaviour from --buildtypeid)")
	pflags.StringVarP(&flags.TC.Build.Parameters, "properties", "p", "", "the TeamCity build parameters to use in 'KEY1=VALUE1;KEY2=VALUE2' format")
	pflags.BoolVarP(&flags.TC.Build.SkipQueue, "skip-queue", "q", false, "Put the build to the queue top")
	pflags.BoolVarP(&flags.TC.Build.Wait, "wait", "w", false, "Wait for the build to complete before tctest exits")
	pflags.BoolVarP(&flags.TC.Build.Latest, "latest", "", false, "gets the latest build in TeamCity")
	pflags.IntVarP(&flags.TC.Build.QueueTimeout, "queue-timeout", "", 60, "How long to wait for a queued build to start running before tctest times out")
	pflags.IntVarP(&flags.TC.Build.RunTimeout, "run-timeout", "", 60, "How long to wait for a running build to finish before tctest times out")
	pflags.BoolVarP(&flags.TC.Build.Comment, "comment", "c", false, "Post a GitHub comment on the PR with test results (adds POST_GITHUB_COMMENT=true property)")
	pflags.BoolVar(&flags.TC.Build.ForceOldUI, "build-link-force-old-ui", false, "Append &fromSakuraUI=true to build URLs to force the classic TeamCity UI")
	pflags.StringSliceVarP(&flags.TC.Build.Tags, "tag", "", []string{}, "TeamCity build tags to add to the triggered build, ie 'tag1,tag2'")
	pflags.IntVar(&flags.TC.Build.MaxBuildsPerPR, "max-builds-per-pr", 5, "maximum number of service builds to trigger per PR (0 = no limit, errors if exceeded)")

	// binding map for viper/pflag -> env
	m := map[string]string{ //nolint:gosec // G101: these are env var names, not credentials
		"server":                           "TCTEST_SERVER",
		"buildtypeid":                      "TCTEST_BUILDTYPEID",
		"build-type-id":                    "TCTEST_BUILD_TYPE_ID",
		"build-type-id-add-service-suffix": "",
		"token-tc":                         "TCTEST_TOKEN_TC",
		"token-gh":                         "GITHUB_TOKEN",
		"username":                         "TCTEST_USER",
		"password":                         "TCTEST_PASS",
		"properties":                       "TCTEST_PROPERTIES",
		"repo":                             "TCTEST_REPO",
		"fileregex":                        "TCTEST_FILEREGEX",
		"acctest-file-suffix-regexes":      "TCTEST_ACCTEST_FILE_SUFFIX_REGEXES",
		"splitteston":                      "TCTEST_SPLIT_TESTS_ON",
		"reappend-split-character":         "TCTEST_REAPPEND_SPLIT_CHARACTER",
		"wait":                             "TCTEST_WAIT",
		"all":                              "",
		"service":                          "",
		"quiet":                            "TCTEST_OUTPUT_QUIET",
		"json":                             "TCTEST_OUTPUT_JSON",
		"silent":                           "TCTEST_OUTPUT_SILENT",
		"dry-run":                          "",
		"verbose":                          "",
		"concurrency":                      "",
		"queue-timeout":                    "",
		"run-timeout":                      "",
		"f-authors":                        "",
		"f-milestone":                      "",
		"f-labels-all":                     "",
		"f-labels-any":                     "",
		"f-created-time":                   "",
		"f-updated-time":                   "",
		"f-title-regex":                    "",
		"f-drafts":                         "",
		"latest":                           "TCTEST_LATESTBUILD",
		"skip-queue":                       "TCTEST_SKIP_QUEUE",
		"open":                             "TCTEST_OPEN_BROWSER",
		"comment":                          "TCTEST_COMMENT",
		"build-link-force-old-ui":          "TCTEST_FORCE_OLD_UI",
		"tag":                              "TCTEST_BUILD_TAGS",
		"max-builds-per-pr":                "",
		"collapse-files-after":             "",
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

// GetFlags returns the fully populated FlagData.
// We must unmarshal from Viper instead of using the globally bound pflags variables
// because pflags only parses command-line arguments. Viper merges environment variables
// (and config files) on top of the CLI flags. By unmarshaling from Viper, we ensure
// that environment variable overrides (e.g. TCTEST_GH_TOKEN) are properly applied.
func GetFlags() FlagData {
	var f FlagData
	if err := viper.Unmarshal(&f); err != nil {
		clog.Log.Fatalf("failed to unmarshal configuration: %v", err)
	}

	// Manually compile Regex fields since Viper doesn't know how to unmarshal strings into *regexp.Regexp natively
	f.DiscoveryConfig.FileRegEx = regexp.MustCompile(viper.GetString("fileregex"))

	suffixStrs := viper.GetStringSlice("acctest-file-suffix-regexes")
	f.DiscoveryConfig.AccTestFileSuffixRegexes = make([]*regexp.Regexp, 0, len(suffixStrs))
	for _, p := range suffixStrs {
		f.DiscoveryConfig.AccTestFileSuffixRegexes = append(f.DiscoveryConfig.AccTestFileSuffixRegexes, regexp.MustCompile(p))
	}

	return f
}

func (cfg DiscoveryConfig) AccTestFileSuffixRegexStrings() string {
	s := make([]string, 0, len(cfg.AccTestFileSuffixRegexes))
	for _, r := range cfg.AccTestFileSuffixRegexes {
		s = append(s, r.String())
	}
	return strings.Join(s, ", ")
}

func (f FlagData) NewTCServer() tc.Server {
	return tc.NewServer(f.TC.ServerURL, f.TC.Token, f.TC.User, f.TC.Pass)
}
