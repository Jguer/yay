## go-alpm

go-alpm is a Go package for binding libalpm. With go-alpm, it becomes possible
to manipulate the Pacman databases and packages just as Pacman would.

This project is MIT Licensed. See LICENSE for details.

## Getting started

1. Import the go-alpm repository in your go script

	import "github.com/jguer/go-alpm"

2. Copy the library to your GOPATH

	mkdir ~/go
	export GOPATH=~/go
	go get github.com/jguer/go-alpm

3. Try the included examples

	cd $GOPATH/src/github.com/jguer/go-alpm/examples
	go run installed.go

## Current Maintainers
* Morganamilo 
* Jguer

## Original Contributors

* Mike Rosset
* Dave Reisner
* Rémy Oudompheng
* Jesus Alvarez
