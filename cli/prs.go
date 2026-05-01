package cli

import (
	"fmt"
	"sort"
	"strings"

	c "github.com/gookit/color" //nolint:misspell
	"github.com/spf13/viper"
)

func (f FlagData) GetAndRunPrsTests(prs map[int]string, testRegExParam string) error {
	// Sort PR numbers to process them in increasing order
	prNumbers := make([]int, 0, len(prs))
	for number := range prs {
		prNumbers = append(prNumbers, number)
	}
	sort.Ints(prNumbers)

	// if --service is specified, bypass test discovery and trigger builds directly
	if len(f.Services) > 0 {
		return f.runServiceBuilds(prNumbers, prs, testRegExParam)
	}

	ok := 0
	for _, number := range prNumbers {
		title := prs[number]
		serviceTests, err := f.GetPrTests(number, title)
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

			// generate test regex if we don't have it
			testRegEx := testRegExParam
			if testRegEx == "" {
				// if no testregex and no tests throw an error (-a is required for all)
				if len(tests) == 0 {
					c.Printf("  %s<red>ERROR:</> no tests found, use TestAcc or --all to run all tests\n", serviceInfo)
					continue
				}

				testRegEx = "(" + strings.Join(tests, "|") + ")"
			}

			// if all tests switch set regex to TestAcc
			if f.RunAllTests {
				testRegEx = "TestAcc"
			}

			// if we have a service put it on the end of the build type id
			buildTypeID := viper.GetString("buildtypeid")
			if s != "" {
				buildTypeID += "_" + strings.ToUpper(s)
			}

			branch := fmt.Sprintf("refs/pull/%d/merge", number)

			if err := GetFlags().BuildCmd(buildTypeID, branch, testRegEx, serviceInfo); err != nil {
				c.Printf("  <red>ERROR: Unable to trigger build:</> %v\n", err)
			}
			fmt.Println()
		}

		ok++
	}
	c.Printf("triggered tests for <yellow>%d</> PRs!\n\n", ok)

	return nil
}

func (f FlagData) runServiceBuilds(prNumbers []int, prs map[int]string, testRegExParam string) error {
	gr := f.NewRepo()

	// resolve which services to run
	services := f.Services

	// fetch valid services from repo for validation or 'all' expansion
	c.Printf("Fetching service list from <cyan>%s/%s</>...\n", gr.Owner, gr.Name)
	validServices, err := gr.ListServices()
	if err != nil {
		return fmt.Errorf("failed to list services: %w", err)
	}
	c.Printf("  found <yellow>%d</> services\n", len(validServices))

	validSet := make(map[string]bool, len(validServices))
	for _, s := range validServices {
		validSet[s] = true
	}

	// handle 'all'
	isAll := len(services) == 1 && strings.EqualFold(services[0], "all")
	if isAll {
		services = validServices
		c.Printf("  using <yellow>all</> services\n")
	} else {
		// validate each specified service
		var invalid []string
		for _, s := range services {
			if !validSet[s] {
				invalid = append(invalid, s)
			}
		}
		if len(invalid) > 0 {
			return fmt.Errorf("invalid service(s): %s\nvalid services: %s", strings.Join(invalid, ", "), strings.Join(validServices, ", "))
		}
	}

	serviceSet := make(map[string]bool, len(services))
	for _, s := range services {
		serviceSet[s] = true
	}

	ok := 0
	for _, number := range prNumbers {
		title := prs[number]

		// if --all is set, skip discovery and trigger TestAcc for each service
		if f.RunAllTests {
			testRegEx := testRegExParam
			if testRegEx == "" {
				testRegEx = "TestAcc"
			}

			c.Printf("PR <cyan>#%d</> %s (--all: running %s)\n", number, title, testRegEx)
			for _, s := range services {
				serviceInfo := "[<yellow>" + s + "</>]"
				buildTypeID := viper.GetString("buildtypeid") + "_" + strings.ToUpper(s)
				branch := fmt.Sprintf("refs/pull/%d/merge", number)

				if err := GetFlags().BuildCmd(buildTypeID, branch, testRegEx, serviceInfo); err != nil {
					c.Printf("  <red>ERROR: Unable to trigger build:</> %v\n", err)
				}
				fmt.Println()
			}
			ok++
			continue
		}

		// normal discovery, but filter to only the specified services
		serviceTests, err := f.GetPrTests(number, title)
		if err != nil {
			c.Printf("  <red>ERROR: discovering tests:</> %v\n\n", err)
			continue
		}

		if serviceTests == nil {
			c.Printf("  <red>ERROR: service tests is nil</>\n\n")
			continue
		}

		for s, tests := range *serviceTests {
			if !serviceSet[s] {
				continue
			}

			serviceInfo := "[<yellow>" + s + "</>]"

			testRegEx := testRegExParam
			if testRegEx == "" {
				if len(tests) == 0 {
					c.Printf("  %s<red>ERROR:</> no tests found, use --all to run all tests\n", serviceInfo)
					continue
				}
				testRegEx = "(" + strings.Join(tests, "|") + ")"
			}

			buildTypeID := viper.GetString("buildtypeid") + "_" + strings.ToUpper(s)
			branch := fmt.Sprintf("refs/pull/%d/merge", number)

			if err := GetFlags().BuildCmd(buildTypeID, branch, testRegEx, serviceInfo); err != nil {
				c.Printf("  <red>ERROR: Unable to trigger build:</> %v\n", err)
			}
			fmt.Println()
		}

		ok++
	}
	c.Printf("triggered tests for <yellow>%d</> PRs across <yellow>%d</> services!\n\n", ok, len(services))

	return nil
}

