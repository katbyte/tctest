package cli

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"syscall"

	c "github.com/gookit/color"
	"github.com/katbyte/tctest/common"
	"golang.org/x/crypto/ssh/terminal"
)

func TcCmd(server, buildTypeId, branch, testRegEx, user, pass string) error {
	c.Printf("triggering <magenta>%s</> for <darkGray>%s...</>\n", branch, testRegEx)
	c.Printf("  <darkGray>%s@%s#%s</>\n", user, server, buildTypeId)

	// prompt for password if not passed in somehow
	if pass == "" {
		fmt.Print("  password:")
		passBytes, err := terminal.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return fmt.Errorf("unable to read in password : %v", err)
		}
		pass = string(passBytes)
		fmt.Println("")
	}

	build, buildUrl, err := TcBuild(server, buildTypeId, branch, testRegEx, user, pass)
	if err != nil {
		return fmt.Errorf("unable to trigger build: %v", err)
	}

	c.Printf("  build <green>%s</> queued! <darkGray>(%s)</>\n", build, buildUrl)

	return nil
}

func TcBuild(server, buildTypeId, branch, testRegEx, user, pass string) (string, string, error) {

	url := fmt.Sprintf("https://%s/app/rest/buildQueue", server)
	body := fmt.Sprintf(`
<build>
	<buildType id="%s"/>
	<properties>
		<property name="BRANCH_NAME" value="%s"/>
		<property name="TEST_PATTERN" value="%s"/>
	</properties>
</build>
`, buildTypeId, branch, testRegEx)

	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return "", "", fmt.Errorf("building build request failed: %v", err)
	}

	req.SetBasicAuth(user, pass)
	req.Header.Set("Content-Type", "application/xml")

	resp, err := common.Http.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("build request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("HTTP status NOT OK: %d", resp.StatusCode)
	}

	data := struct {
		BuildId string `xml:"id,attr"`
	}{}
	if err := xml.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", "", fmt.Errorf("unable to decode XML: %d", resp.StatusCode)
	}

	return data.BuildId, fmt.Sprintf("https://%s/viewQueued.html?itemId=%s", server, data.BuildId), nil
}
