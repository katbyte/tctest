package main

import (
	"log"
	"os"

	"github.com/katbyte/tctest/cmd"
)

func main() {
	if err := cmd.Make().Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}

	os.Exit(0)
}
