package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/leanote/leanote/app/tests/harness"
)

func main() {
	if len(os.Args) != 2 || (os.Args[1] != "up" && os.Args[1] != "down") {
		fmt.Fprintln(os.Stderr, "usage: go run ./app/tests/harness/cmd/env <up|down>")
		os.Exit(2)
	}

	root, err := filepath.Abs(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	env := harness.NewMongoEnvironment(root)
	if os.Args[1] == "down" {
		err = env.Down()
	} else {
		err = env.Up()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
