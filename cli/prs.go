package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/katbyte/tctest/lib/clog"
	"github.com/katbyte/tctest/lib/cout"
)

func (f *FlagData) GetAndRunPrsTests(prs map[int]string, testRegExParam string) error {
	// Sort PR numbers to process them in increasing order
	prNumbers := make([]int, 0, len(prs))
	for number := range prs {
		prNumbers = append(prNumbers, number)
	}
	sort.Ints(prNumbers)

	// if --service is specified, resolve and validate services up front
	serviceFilter, err := f.resolveServiceFilter()
	if err != nil {
		return err
	}

	// the direct-trigger path (--service + --all/test regex) triggers one build per service,
	// so enforce --max-builds-per-pr up front
	if serviceFilter != nil && (f.RunAllTests || testRegExParam != "") && f.TC.Build.MaxBuildsPerPR > 0 && len(serviceFilter.services) > f.TC.Build.MaxBuildsPerPR {
		return fmt.Errorf("--service would trigger %d builds per PR, exceeding --max-builds-per-pr limit of %d (use --max-builds-per-pr 0 for no limit)", len(serviceFilter.services), f.TC.Build.MaxBuildsPerPR)
	}

	ok := 0
	failed := 0
	buildsTriggered := 0
	buildsFailed := 0
	servicesSkipped := 0
	for _, number := range prNumbers {
		title := prs[number]

		// when --service + (--all or explicit test_regex), skip discovery and trigger directly
		if serviceFilter != nil && (f.RunAllTests || testRegExParam != "") {
			testRegEx := testRegExParam
			if testRegEx == "" {
				testRegEx = "TestAcc"
			}

			cout.Printf("PR <cyan>#%d</> %s (running %s)\n", number, title, testRegEx)

			// discovery validates the PR as a side effect; here we skip discovery, so check
			// the PR is open and mergeable before triggering builds on its merge ref
			if err := f.CheckPrCanBuild(number); err != nil {
				cout.Errorf("  <red>ERROR:</> %v\n\n", err)
				failed++
				continue
			}

			for _, s := range serviceFilter.services {
				if err := f.triggerServiceBuild(s, number, testRegEx); err != nil {
					buildsFailed++
				} else {
					buildsTriggered++
				}
			}
			ok++
			continue
		}

		// discover tests from PR files
		serviceTests, err := f.GetPrTests(number, title)
		if err != nil {
			cout.Errorf("  <red>ERROR: discovering tests:</> %v\n\n", err)
			failed++
			continue
		}

		if serviceTests == nil {
			cout.Errorf("  <red>ERROR: service tests is nil</>\n\n")
			failed++
			continue
		}

		// check max-builds-per-pr limit
		if f.TC.Build.MaxBuildsPerPR > 0 {
			serviceCount := 0
			for s := range serviceTests {
				if serviceFilter != nil && !serviceFilter.set[s] {
					continue
				}
				serviceCount++
			}
			if serviceCount > f.TC.Build.MaxBuildsPerPR {
				cout.Errorf("  <red>ERROR:</> would trigger <yellow>%d</> service builds, exceeding --max-builds-per-pr limit of <yellow>%d</>\n\n", serviceCount, f.TC.Build.MaxBuildsPerPR)
				failed++
				continue
			}
		}

		// trigger a build for each service
		prBuilds := 0
		prFailed := 0
		for s, tests := range serviceTests {
			// if --service is set, skip services not in the filter
			if serviceFilter != nil && !serviceFilter.set[s] {
				servicesSkipped++
				clog.Log.Debugf("  skipping service %s (not in --service filter)", s)
				continue
			}

			serviceInfo := ""
			if s != "" {
				serviceInfo = "[<yellow>" + s + "</>]"
			}

			// generate the test regex: --all wins, then an explicit regex, then discovered tests (+ --add-tests)
			testRegEx := testRegExParam
			switch {
			case f.RunAllTests:
				testRegEx = "TestAcc"
			case testRegEx == "":
				allTests := append([]string{}, tests...)
				allTests = append(allTests, f.AddTests...)

				if len(allTests) == 0 {
					cout.Errorf("  %s<red>ERROR:</> no tests found, use TestAcc or --all to run all tests\n", serviceInfo)
					continue
				}

				testRegEx = "(" + strings.Join(allTests, "|") + ")"
			}

			if err := f.triggerServiceBuild(s, number, testRegEx); err != nil {
				buildsFailed++
				prFailed++
				continue
			}
			buildsTriggered++
			prBuilds++
		}

		if serviceFilter != nil && prBuilds == 0 && prFailed == 0 {
			cout.Printf("  <yellow>no matching services</> for --service filter (discovered services had no overlap)\n\n")
		}

		ok++
	}

	// summary
	cout.Printf("triggered <yellow>%d</> build(s) for <yellow>%d</> PR(s)", buildsTriggered, ok)
	if buildsFailed > 0 {
		cout.Printf(" <red>(%d build(s) failed to trigger)</>", buildsFailed)
	}
	if servicesSkipped > 0 {
		cout.Printf(" <darkGray>(%d service(s) skipped by --service filter)</>", servicesSkipped)
	}
	cout.Printf("\n\n")

	cout.FlushJSON()

	if failed > 0 {
		return fmt.Errorf("%d of %d PRs failed", failed, len(prNumbers))
	}
	if buildsFailed > 0 {
		return fmt.Errorf("%d build(s) failed to trigger", buildsFailed)
	}

	return nil
}

// serviceFilterResult holds the resolved and validated service filter
type serviceFilterResult struct {
	services []string        // ordered list of services
	set      map[string]bool // set for fast lookup
}

// resolveServiceFilter validates --service values against the GitHub repo. Returns nil if --service is not set.
func (f *FlagData) resolveServiceFilter() (*serviceFilterResult, error) {
	if len(f.Services) == 0 {
		return nil, nil
	}

	ghr := f.NewRepo()

	cout.Printf("Fetching service list from <cyan>%s/%s</>...\n", ghr.Owner, ghr.Name)
	validServices, err := ghr.ListServices()
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}
	cout.Printf("  found <yellow>%d</> services\n", len(validServices))

	validSet := make(map[string]bool, len(validServices))
	for _, s := range validServices {
		validSet[s] = true
	}

	// handle 'all'
	services := f.Services
	if len(services) == 1 && strings.EqualFold(services[0], "all") {
		services = validServices
		cout.Printf("  using <yellow>all</> services\n")
	} else {
		// validate each specified service
		var invalid []string
		for _, s := range services {
			if !validSet[s] {
				invalid = append(invalid, s)
			}
		}
		if len(invalid) > 0 {
			return nil, fmt.Errorf("invalid service(s): %s\nvalid services: %s", strings.Join(invalid, ", "), strings.Join(validServices, ", "))
		}
	}

	set := make(map[string]bool, len(services))
	for _, s := range services {
		set[s] = true
	}

	return &serviceFilterResult{services: services, set: set}, nil
}

// triggerServiceBuild triggers a build for a single service on a PR
func (f *FlagData) triggerServiceBuild(service string, prNumber int, testRegEx string) error {
	serviceInfo := ""
	if service != "" {
		serviceInfo = "[" + service + "]"
	}

	buildTypeID := f.TC.Build.TypeID
	if service != "" && f.TC.Build.AddServiceSuffix {
		buildTypeID += "_" + strings.ToUpper(service)
	}

	branch := fmt.Sprintf("refs/pull/%d/merge", prNumber)

	buildID, buildURL, err := f.BuildCmd(buildTypeID, branch, testRegEx, serviceInfo)
	if err != nil {
		cout.Errorf("  <red>ERROR: Unable to trigger build:</> %v\n", err)
		cout.Println()
		return err
	}

	// dry-run triggers nothing, so don't emit machine-readable records for it
	if !f.DryRun {
		cout.Quietf("%d@%s@%d %s\n", prNumber, service, buildID, buildURL)
		cout.AddResult(prNumber, service, buildID, buildURL)
	}

	cout.Quietf("%d@%s@%d %s\n", prNumber, service, buildID, buildURL)
	cout.AddResult(prNumber, service, buildID, buildURL)
	cout.Println()
	return nil
}
