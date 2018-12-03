[![GPL3 license](https://img.shields.io/badge/License-GPL3-blue.svg)](LICENSE)
[![GoDoc](https://godoc.org/github.com/Morganamilo/go-srcinfo?status.svg)](https://godoc.org/github.com/Morganamilo/go-srcinfo)
[![Build Status](https://travis-ci.org/Morganamilo/go-srcinfo.svg?branch=master)](https://travis-ci.org/Morganamilo/go-srcinfo)
[![codecov](https://codecov.io/gh/Morganamilo/go-srcinfo/branch/master/graph/badge.svg)](https://codecov.io/gh/Morganamilo/go-srcinfo)
[![Go Report Card](https://goreportcard.com/badge/github.com/Morganamilo/go-srcinfo)](https://goreportcard.com/report/github.com/Morganamilo/go-srcinfo)

# go-srcinfo

A golang package for parsing `.SRCINFO` files. [SRCINFO](https://wiki.archlinux.org/index.php/.SRCINFO)

go-srcinfo aimes to be simple while ensuring each srcinfo is syntactically
correct. Split packages and architecture specific fields are fully supported.

# Examples

Reading a srcinfo from a file
```go
package main

import (
	"fmt"
	"github.com/Morganamilo/go-srcinfo"
)

func main() {
	info, err := srcinfo.ParseFile("SRCINFO")
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(info)
}
```

Reading each package from a split package
```go
package main

import (
	"fmt"
	"github.com/Morganamilo/go-srcinfo"
)

func main() {
	info, err := srcinfo.ParseFile("SRCINFO")
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, pkg := range info.SplitPackages() {
		fmt.Printf("%s-%s: %s\n", pkg.Pkgname, info.Version(), pkg.Pkgdesc)
	}
}
```

Showing the architecture of each source
```go
package main

import (
	"fmt"
	"github.com/Morganamilo/go-srcinfo"
)

func main() {
	info, err := srcinfo.ParseFile("SRCINFO")
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, source := range info.Source {
		if source.Arch == "" {
			fmt.Printf("This source is for %s: %s\n", "any", source.Value)
		} else {
			fmt.Printf("This source is for %s: %s\n", source.Arch, source.Value)
		}
	}
}
```

Reading a srcinfo from a string
```go
package main

import (
	"fmt"
	"github.com/Morganamilo/go-srcinfo"
)

const str = `
pkgbase = gdc-bin
	pkgver = 6.3.0+2.068.2
	pkgrel = 1
	url = https://gdcproject.org/
	arch = i686
	arch = x86_64
	license = GPL
	source_i686 = http://gdcproject.org/downloads/binaries/6.3.0/i686-linux-gnu/gdc-6.3.0+2.068.2.tar.xz
	md5sums_i686 = cc8dcd66b189245e39296b1382d0dfcc
	source_x86_64 = http://gdcproject.org/downloads/binaries/6.3.0/x86_64-linux-gnu/gdc-6.3.0+2.068.2.tar.xz
	md5sums_x86_64 = 16d3067ebb3938dba46429a4d9f6178f

pkgname = gdc-bin
	pkgdesc = Compiler for D programming language which uses gcc backend
	depends = gdc-gcc
	depends = perl
	depends = binutils
	depends = libgphobos
	provides = d-compiler=2.068.2
	provides = gdc=6.3.0+2.068.2

pkgname = gdc-gcc
	pkgdesc = The GNU Compiler Collection - C and C++ frontends (from GDC, gdcproject.org)
	provides = gcc=6.3.0
	provides = gcc-libs=6.3.0

pkgname = libgphobos-lib32
	pkgdesc = Standard library for D programming language, GDC port
	provides = d-runtime-lib32
	provides = d-stdlib-lib32
`
func main() {
	info, err := srcinfo.Parse(str)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(info)
}
```


