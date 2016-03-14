package main

import (
	"fmt"
	"testing"
)

func TestFormatPushURL(t *testing.T) {
	testAccessToken := "7777"
	testUsername := "roonyh"
	testURL := "https://github.com/test/name"

	pushURL := formatPushURL(testAccessToken, testUsername, testURL)
	expectedURL := "https://roonyh:7777@github.com/test/name"

	if pushURL != expectedURL {
		fmt.Println("expected:", expectedURL, "got:", pushURL)
		t.Fail()
	}
}

func TestGetPushBranch(t *testing.T) {
	tests := []struct {
		URL      string
		Expected string
	}{
		{"http://github.com/roonyh/roonyh.github.io", "master"},
		{"http://github.com/cliffrange/roonyh.github.io", "gh-pages"},
		{"http://github.com/roonyh/some-stupid-project", "gh-pages"},
	}

	for _, test := range tests {
		got := getPushBranch(test.URL)
		if got != test.Expected {
			fmt.Println("expected:", test.Expected, "for", test.URL, "got", got)
			t.Fail()
		}
	}
}
