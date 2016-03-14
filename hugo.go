package main

import (
	"bytes"
	"log"
	"os/exec"
	"strings"
)

// HugoBuild builds the site in the path
func HugoBuild(path string) (string, error) {
	cmd := exec.Command("hugo", "-s", path)
	cmd.Stdin = strings.NewReader("some input")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	log.Println(err)
	if err != nil {
		log.Println(out.String())
	}
	return out.String(), err
}
