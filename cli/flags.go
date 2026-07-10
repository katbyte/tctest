package cli

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// resolveBuildTypeID handles the legacy --buildtypeid to --build-type-id migration.
// It errors if both are set. When only the old flag is used, it copies the value to
// build-type-id and enables build-type-id-add-service-suffix to maintain the old behaviour.
// Called from PersistentPreRunE before ValidateParams so the resolved value is available for validation.
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
	GH              FlagsGitHub
	TC              FlagsTeamCity
	DiscoveryConfig DiscoveryConfig
	OpenInBrowser   bool
	RunAllTests     bool
	Services        []string
	Quiet           bool
	JSON            bool
	Silent          bool
	DryRun          bool
}

type DiscoveryConfig struct {
	FileRegExStr             string
	SplitTestsOn             string
	ReappendSplitCharacter   bool
	AccTestFileSuffixRegexes []string
	Concurrency              int
	LocalRepoPath            string
	LocalTraceDepth          int
	LocalVendorMode          string
	LocalMode                string
}

type FlagsGitHub struct {
	Token     string
	Repo      string
	FilterPRs FlagsGitHubPrFilter
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
	TypeID           string
	LegacyTypeID     string // deprecated --buildtypeid, resolved in resolveBuildTypeID()
	Parameters       string
	SkipQueue        bool
	Wait             bool
	Latest           bool
	Comment          bool
	ForceOldUI       bool
	AddServiceSuffix bool
	QueueTimeout     int
	RunTimeout       int
	MaxBuildsPerPR   int
	Tags             []string
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

	// "services?" matches both provider layouts: AWS(`service`) and Azure(`services`).
	pflags.StringVar(&flags.DiscoveryConfig.FileRegExStr, "fileregex", `^internal/services?/[^/]+/[a-z0-9_][^/]*$`, "the regex to filter files by")
	pflags.StringVar(&flags.DiscoveryConfig.SplitTestsOn, "splitteston", "_", "the character to split tests on and use the value on the left")
	pflags.StringSliceVar(&flags.DiscoveryConfig.AccTestFileSuffixRegexes, "acctest-file-suffix-regexes", []string{
		`^_resource.*_test$`,   // Azure, this will also covers test files like `linux_virtual_machine_scale_set_resource_auth_test.go`
		`^_test$`,              // both providers
		`^_list_test$`,         // AWS list data-source tests
		`^_identity_gen_test$`, // AWS generated identity tests
		`^_tags_gen_test$`,     // AWS generated tags tests
		`^_data_source_test$`,  // data-source tests (both providers)
	}, "comma-separated list of regex patterns to match acceptance test filenames suffix (without '.go')")
	pflags.BoolVar(&flags.DiscoveryConfig.ReappendSplitCharacter, "reappend-split-character", false, "whether to append the split character to the resulting test filter for more precise filtering")
	pflags.IntVar(&flags.DiscoveryConfig.Concurrency, "concurrency", 5, "maximum number of concurrent file downloads during test discovery")
	pflags.StringVar(&flags.DiscoveryConfig.LocalRepoPath, "local-repo-path", "", "path to a local git clone for AST-based test detection (enables import tracing from helper files)")
	pflags.IntVar(&flags.DiscoveryConfig.LocalTraceDepth, "local-trace-depth", 10, "how many levels of import tracing to perform for helper file changes (0 to disable)")
	pflags.StringVar(&flags.DiscoveryConfig.LocalVendorMode, "local-vendor-mode", "basic", "mode for vendor AST detection: 'basic' (package-based import tracing) or 'none' (disabled)")
	pflags.StringVar(&flags.DiscoveryConfig.LocalMode, "local-mode", "AST", "mode for local test detection: 'off' (or empty, uses default web mode), 'AST' (the new ast mode). Note: 'SSA' (super slow analyse) to be added in the future")

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
		"concurrency":                      "",
		"local-repo-path":                  "TCTEST_LOCAL_REPO_PATH",
		"local-trace-depth":                "",
		"local-vendor-mode":                "TCTEST_LOCAL_VENDOR_MODE",
		"local-mode":                       "TCTEST_LOCAL_MODE",
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
		Services:      viper.GetStringSlice("service"),
		Quiet:         viper.GetBool("quiet"),
		JSON:          viper.GetBool("json"),
		Silent:        viper.GetBool("silent"),
		DryRun:        viper.GetBool("dry-run"),
		DiscoveryConfig: DiscoveryConfig{
			FileRegExStr:             viper.GetString("fileregex"),
			SplitTestsOn:             viper.GetString("splitteston"),
			ReappendSplitCharacter:   viper.GetBool("reappend-split-character"),
			AccTestFileSuffixRegexes: viper.GetStringSlice("acctest-file-suffix-regexes"),
			Concurrency:              viper.GetInt("concurrency"),
			LocalRepoPath:            viper.GetString("local-repo-path"),
			LocalTraceDepth:          viper.GetInt("local-trace-depth"),
			LocalVendorMode:          viper.GetString("local-vendor-mode"),
			LocalMode:                viper.GetString("local-mode"),
		},
		GH: FlagsGitHub{
			Repo:  viper.GetString("repo"),
			Token: viper.GetString("token-gh"),
			FilterPRs: FlagsGitHubPrFilter{
				Authors:      viper.GetStringSlice("f-authors"),
				LabelsOr:     viper.GetStringSlice("f-labels-any"),
				LabelsAnd:    viper.GetStringSlice("f-labels-all"),
				Milestone:    viper.GetString("f-milestone"),
				TitleRegex:   viper.GetString("f-title-regex"),
				CreationTime: viper.GetDuration("f-created-time"),
				UpdatedTime:  viper.GetDuration("f-updated-time"),
				Drafts:       viper.GetBool("f-drafts"),
			},
		},
		TC: FlagsTeamCity{
			ServerURL: viper.GetString("server"),
			Token:     viper.GetString("token-tc"),
			User:      viper.GetString("username"),
			Pass:      viper.GetString("password"),
			Build: FlagsTeamCityBuild{
				TypeID:           viper.GetString("build-type-id"),
				Parameters:       viper.GetString("properties"),
				SkipQueue:        viper.GetBool("skip-queue"),
				Wait:             viper.GetBool("wait"),
				Latest:           viper.GetBool("latest"),
				Comment:          viper.GetBool("comment"),
				ForceOldUI:       viper.GetBool("build-link-force-old-ui"),
				AddServiceSuffix: viper.GetBool("build-type-id-add-service-suffix"),
				QueueTimeout:     viper.GetInt("queue-timeout"),
				RunTimeout:       viper.GetInt("run-timeout"),
				MaxBuildsPerPR:   viper.GetInt("max-builds-per-pr"),
				Tags:             viper.GetStringSlice("tag"),
			},
		},
	}
}
