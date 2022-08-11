package cli

import (
	"fmt"
	"strings"

	c "github.com/gookit/color" //nolint:misspell
	"github.com/spf13/viper"
)

func (f FlagData) GetAndRunPrsTests(prs []int, testRegExParam string) error {
	for _, pri := range prs {

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

			branch := fmt.Sprintf("refs/pull/%d/merge", pri)

			if err := GetFlags().BuildCmd(buildTypeID, branch, testRegEx, serviceInfo); err != nil {
				c.Printf("  <red>ERROR: Unable to trigger build:</> %v\n", err)
			}
			fmt.Println()
		}
	}

	return nil
}
