package main

import (
	"os"

	"github.com/spf13/hugo/commands"
)

// Build builds the site in the path
func HugoBuild(path string) {
	os.Args = append(os.Args, "-s", path)
	commands.Execute()
}
