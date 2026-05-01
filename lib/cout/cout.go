package cout

import (
	"io"
	"os"

	c "github.com/gookit/color" //nolint:misspell
)

// Verbosity levels
const (
	VerbosityNormal = iota
	VerbosityQuiet
	VerbositySilent
)

// Level controls the output verbosity. Set before any output calls.
var Level = VerbosityNormal

// Writer returns the appropriate writer for normal output (os.Stdout or discard)
func Writer() io.Writer {
	if Level >= VerbosityQuiet {
		return io.Discard
	}
	return os.Stdout
}

// Printf prints normal output with color support (suppressed in quiet and silent modes)
func Printf(format string, args ...interface{}) {
	if Level >= VerbosityQuiet {
		return
	}
	c.Printf(format, args...)
}

// Println prints normal output (suppressed in quiet and silent modes)
func Println(args ...interface{}) {
	if Level >= VerbosityQuiet {
		return
	}
	c.Println(args...)
}

// Quietf prints output in quiet mode with color support (suppressed only in silent mode).
// Use this for the minimal machine-readable output.
func Quietf(format string, args ...interface{}) {
	if Level >= VerbositySilent {
		return
	}
	c.Printf(format, args...)
}
