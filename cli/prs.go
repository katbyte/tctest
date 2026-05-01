package cli

import (
	"fmt"
	"sort"
	"strings"

	c "github.com/gookit/color" //nolint:misspell
	"github.com/katbyte/tctest/lib/cout"
	"github.com/spf13/viper"
)

func (f FlagData) GetAndRunPrsTests(prs map[int]string, testRegExParam string) error {
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

	ok := 0
	for _, number := range prNumbers {
		title := prs[number]

		// when --service + --all, skip discovery and trigger TestAcc for each service directly
		if serviceFilter != nil && f.RunAllTests {
			testRegEx := testRegExParam
			if testRegEx == "" {
				testRegEx = "TestAcc"
			}

			cout.Printf("PR <cyan>#%d</> %s (--all: running %s)\n", number, title, testRegEx)
			for _, s := range serviceFilter.services {
				f.triggerServiceBuild(s, number, testRegEx)
			}
			ok++
			continue
		}

		// discover tests from PR files
		serviceTests, err := f.GetPrTests(number, title)
		if err != nil {
			c.Printf("  <red>ERROR: discovering tests:</> %v\n\n", err)
			continue
		}

		if serviceTests == nil {
			c.Printf("  <red>ERROR: service tests is nil</>\n\n")
			continue
		}

		// trigger a build for each service
		for s, tests := range *serviceTests {
			// if --service is set, skip services not in the filter
			if serviceFilter != nil && !serviceFilter.set[s] {
				continue
			}

			serviceInfo := ""
			if s != "" {
				serviceInfo = "[<yellow>" + s + "</>]"
			}

			// generate test regex if we don't have it
			testRegEx := testRegExParam
			if testRegEx == "" {
				if len(tests) == 0 {
					c.Printf("  %s<red>ERROR:</> no tests found, use TestAcc or --all to run all tests\n", serviceInfo)
					continue
				}

				testRegEx = "(" + strings.Join(tests, "|") + ")"
			}

			// if --all set regex to TestAcc
			if f.RunAllTests {
				testRegEx = "TestAcc"
			}

			f.triggerServiceBuild(s, number, testRegEx)
		}

		ok++
	}

	if serviceFilter != nil {
		cout.Printf("triggered tests for <yellow>%d</> PRs across <yellow>%d</> services!\n\n", ok, len(serviceFilter.services))
	} else {
		cout.Printf("triggered tests for <yellow>%d</> PRs!\n\n", ok)
	}

	cout.FlushJSON()

	return nil
}

// serviceFilterResult holds the resolved and validated service filter
type serviceFilterResult struct {
	services []string        // ordered list of services
	set      map[string]bool // set for fast lookup
}

// resolveServiceFilter validates --service values against the GitHub repo. Returns nil if --service is not set.
func (f FlagData) resolveServiceFilter() (*serviceFilterResult, error) {
	if len(f.Services) == 0 {
		return nil, nil
	}

	gr := f.NewRepo()

	cout.Printf("Fetching service list from <cyan>%s/%s</>...\n", gr.Owner, gr.Name)
	validServices, err := gr.ListServices()
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
func (f FlagData) triggerServiceBuild(service string, prNumber int, testRegEx string) {
	serviceInfo := ""
	if service != "" {
		serviceInfo = "[" + service + "]"
	}

	buildTypeID := viper.GetString("build-type-id")
	if service != "" && viper.GetBool("build-type-id-add-service-suffix") {
		buildTypeID += "_" + strings.ToUpper(service)
	}

	branch := fmt.Sprintf("refs/pull/%d/merge", prNumber)

	buildID, buildURL, err := GetFlags().BuildCmd(buildTypeID, branch, testRegEx, serviceInfo)
	if err != nil {
		c.Printf("  <red>ERROR: Unable to trigger build:</> %v\n", err)
	} else {
		cout.Quietf("%d@%s@%d %s\n", prNumber, service, buildID, buildURL)
		cout.AddResult(prNumber, service, buildID, buildURL)
	}
	cout.Println()
}
