package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseTestFile(t *testing.T) {
	// Define the base config for testing
	cfg := DiscoveryConfig{
		SplitTestsOn:           "",
		ReappendSplitCharacter: false,
	}

	tests := []struct {
		name          string
		filePath      string
		splitOn       string
		reappendSplit bool
		wantService   string
		wantTests     []string
	}{
		{
			name:        "azurerm basic test file",
			filePath:    "testdata/providers/azurerm/internal/services/mockservice/mockservice_resource_test.go",
			wantService: "mockservice",
			wantTests: []string{
				"TestAccMockService_basic",
				"TestAccMockService_update",
			},
		},
		{
			name:        "azurerm list test file with split",
			filePath:    "testdata/providers/azurerm/internal/services/mockservice/mockservice_list_test.go",
			splitOn:     "_",
			reappendSplit: false,
			wantService: "mockservice",
			wantTests: []string{
				"TestAccMockServiceList", // basic is chopped off because of splitOn
			},
		},
		{
			name:        "aws test file",
			filePath:    "testdata/providers/aws/internal/service/fakeservice/fake_resource_test.go",
			wantService: "", // Note: AWS uses /service/ not /services/ so our simplistic split fails. This documents current behavior.
			wantTests: []string{
				"TestAccFakeResource_basic",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "cli", tt.filePath))
			if err != nil {
				// try local path directly if running inside cli/
				content, err = os.ReadFile(tt.filePath)
				if err != nil {
					t.Fatalf("failed to read test file %s: %v", tt.filePath, err)
				}
			}

			// Override config for this test case
			testCfg := cfg
			testCfg.SplitTestsOn = tt.splitOn
			testCfg.ReappendSplitCharacter = tt.reappendSplit

			service, testNames, err := parseTestFile(content, tt.filePath, testCfg)
			if err != nil {
				t.Fatalf("parseTestFile() error = %v", err)
			}

			if service != tt.wantService {
				t.Errorf("parseTestFile() service = %v, want %v", service, tt.wantService)
			}

			if !reflect.DeepEqual(testNames, tt.wantTests) {
				t.Errorf("parseTestFile() tests = %v, want %v", testNames, tt.wantTests)
			}
		})
	}
}
