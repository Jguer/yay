package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsing(t *testing.T) {
	type source struct {
		URL       string
		Branch    string
		Protocols []string
	}

	urls := []string{
		"git+https://github.com/neovim/neovim.git",
		"git://github.com/jguer/yay.git#branch=master",
		"git://github.com/davidgiven/ack",
		"git://github.com/jguer/yay.git#tag=v3.440",
		"git://github.com/jguer/yay.git#commit=e5470c88c6e2f9e0f97deb4728659ffa70ef5d0c",
		"a+b+c+d+e+f://github.com/jguer/yay.git#branch=foo",
	}

	sources := []source{
		{"github.com/neovim/neovim.git", "HEAD", []string{"https"}},
		{"github.com/jguer/yay.git", "master", []string{"git"}},
		{"github.com/davidgiven/ack", "HEAD", []string{"git"}},
		{"", "", nil},
		{"", "", nil},
		{"", "", nil},
	}

	for n, url := range urls {
		url, branch, protocols := parseSource(url)
		compare := sources[n]

		assert.Equal(t, compare.URL, url)
		assert.Equal(t, compare.Branch, branch)
		assert.Equal(t, compare.Protocols, protocols)
	}
}
