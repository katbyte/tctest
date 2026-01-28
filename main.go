package main

import (
	"os"
	//nolint:misspell
	c "github.com/gookit/color"
	"github.com/katbyte/tctest/cli"
	"github.com/katbyte/tctest/lib/clog"
)

func main() {
	cmd, err := cli.Make()
	if err != nil {
		clog.Log.Error(c.Sprintf("<red>tctest: building cmd</> %v", err))

		os.Exit(1)
	}

	if err := cmd.Execute(); err != nil {
		clog.Log.Error(c.Sprintf("<red>tctest:</> %v", err))

		os.Exit(1)
	}

	os.Exit(0)
}
