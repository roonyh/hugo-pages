package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestHugoBuild(t *testing.T) {
	out, err := HugoBuild("./manual-test/test-blog-1")
	if err != nil {
		fmt.Println("HugoBuild gives an error")
		t.Fail()
	}
	if len(out) == 0 {
		fmt.Println("HugoBuild doesnt log")
		t.Fail()
	}
}

func TestHugoBuildEmpty(t *testing.T) {
	out, err := HugoBuild("./manual-test/test-blog-3")
	if err == nil {
		fmt.Println("Does not get the correct error")
		t.Fail()
	}
	if !strings.Contains(out, "Unable to locate") {
		fmt.Println("HugoBuild doesnt log correct ouput")
		t.Fail()
	}
}
