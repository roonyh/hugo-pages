package main

import (
	"fmt"
	"testing"
)

func TestFormatPushURL(t *testing.T) {
	testAccessToken := "7777"
	testUsername := "roonyh"
	testFullname := "test/name"

	pushURL := formatPushURL(testAccessToken, testUsername, testFullname)
	expectedURL := "https://roonyh:7777@github.com/test/name"

	if pushURL != expectedURL {
		fmt.Println("expected:", expectedURL, "got:", pushURL)
		t.Fail()
	}
}
