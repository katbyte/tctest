package cout

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	c "github.com/gookit/color" //nolint:misspell
)

// Verbosity levels (ordered from least to most output)
const (
	VerbositySilent = iota
	VerbosityJSON
	VerbosityQuiet
	VerbosityNormal
	VerbosityVerbose
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
	if Level < VerbosityNormal {
		return io.Discard
	}
	return os.Stdout
}

// Printf prints normal output with color support (suppressed in quiet, json, and silent modes)
func Printf(format string, args ...interface{}) {
	if Level < VerbosityNormal {
		return
	}
	c.Printf(format, args...)
}

// Println prints normal output (suppressed in quiet, json, and silent modes)
func Println(args ...interface{}) {
	if Level < VerbosityNormal {
		return
	}
	c.Println(args...)
}

// Quietf prints output only in quiet mode with color support.
// Use this for the minimal machine-readable output.
func Quietf(format string, args ...interface{}) {
	if Level != VerbosityQuiet {
		return
	}
	c.Printf(format, args...)
}

// Verbosef prints detailed output only when -v is set (suppressed at normal and below).
func Verbosef(format string, args ...interface{}) {
	if Level < VerbosityVerbose {
		return
	}
	c.Printf(format, args...)
}
