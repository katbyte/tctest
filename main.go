package main

import (
	"os"

	c "github.com/gookit/color"
	"github.com/katbyte/tctest/cmd"
	"github.com/katbyte/tctest/common"
)

func main() {
	if err := cmd.Make().Execute(); err != nil {
		common.Log.Errorf(c.Sprintf("<red>tctest:</> %v", err))
		os.Exit(1)
	}

	os.Exit(0)
}
