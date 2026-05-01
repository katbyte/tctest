package cout

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	c "github.com/gookit/color" //nolint:misspell
)

// Verbosity levels
const (
	VerbosityNormal = iota
	VerbosityQuiet
	VerbosityJSON
	VerbositySilent
)

// Level controls the output verbosity. Set before any output calls.
var Level = VerbosityNormal

// BuildResult represents a single triggered build for JSON output
type BuildResult struct {
	PR          int    `json:"pr"`
	Service     string `json:"service"`
	BuildNumber int    `json:"build_number"`
	URL         string `json:"url"`
}

// jsonResults collects build results for JSON output
var jsonResults []BuildResult

// AddResult collects a build result for JSON output
func AddResult(pr int, service string, buildNumber int, url string) {
	jsonResults = append(jsonResults, BuildResult{
		PR:          pr,
		Service:     service,
		BuildNumber: buildNumber,
		URL:         url,
	})
}

// FlushJSON outputs collected results as a JSON array and resets the collector
func FlushJSON() {
	if Level != VerbosityJSON || len(jsonResults) == 0 {
		return
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(jsonResults); err != nil {
		fmt.Fprintf(os.Stderr, "error marshalling JSON: %v\n", err)
	}
	jsonResults = nil
}

// Writer returns the appropriate writer for normal output (os.Stdout or discard)
func Writer() io.Writer {
	if Level >= VerbosityQuiet {
		return io.Discard
	}
	return os.Stdout
}

// Printf prints normal output with color support (suppressed in quiet, json, and silent modes)
func Printf(format string, args ...interface{}) {
	if Level >= VerbosityQuiet {
		return
	}
	c.Printf(format, args...)
}

// Println prints normal output (suppressed in quiet, json, and silent modes)
func Println(args ...interface{}) {
	if Level >= VerbosityQuiet {
		return
	}
	c.Println(args...)
}

// Quietf prints output in quiet mode with color support (suppressed in json and silent modes).
// Use this for the minimal machine-readable output.
func Quietf(format string, args ...interface{}) {
	if Level >= VerbosityJSON {
		return
	}
	c.Printf(format, args...)
}
