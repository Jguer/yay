package github

import (
	"testing"
)

func TestParsing(t *testing.T) {
	type source struct {
		sourceurl string
		owner     string
		repo      string
	}

	neovim := source{sourceurl: "git+https://github.com/neovim/neovim.git"}
	neovim.owner, neovim.repo = parseSource(neovim.sourceurl)

	if neovim.owner != "neovim" || neovim.repo != "neovim" {
		t.Fatalf("Expected to find neovim/neovim, found %+v/%+v", neovim.owner, neovim.repo)
	}

	yay := source{sourceurl: "git://github.com/jguer/yay.git#branch=master"}
	yay.owner, yay.repo = parseSource(yay.sourceurl)
	if yay.owner != "jguer" || yay.repo != "yay" {
		t.Fatalf("Expected to find jguer/yay, found %+v/%+v", yay.owner, yay.repo)
	}

	ack := source{sourceurl: "git://github.com/davidgiven/ack"}
	ack.owner, ack.repo = parseSource(ack.sourceurl)
	if ack.owner != "davidgiven" || ack.repo != "ack" {
		t.Fatalf("Expected to find davidgiven/ack, found %+v/%+v", ack.owner, ack.repo)
	}

}
