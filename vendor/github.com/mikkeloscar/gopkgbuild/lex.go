// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// based on the lexer from: src/pkg/text/template/parse/lex.go (golang source)

package pkgbuild

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// pos is a position in input being scanned
type pos int

type item struct {
	typ itemType
	pos pos
	val string
}

func (i item) String() string {
	switch {
	case i.typ == itemEOF:
		return "EOF"
	case i.typ == itemError:
		return i.val
	case len(i.val) > 10:
		return fmt.Sprintf("%.10q...", i.val)
	}
	return fmt.Sprintf("%q", i.val)
}

type itemType int

const (
	itemError itemType = iota
	itemEOF
	itemVariable
	itemValue
	itemEndSplit
	// PKGBUILD variables
	itemPkgname      // pkgname variable
	itemPkgver       // pkgver variable
	itemPkgrel       // pkgrel variable
	itemPkgdir       // pkgdir variable
	itemEpoch        // epoch variable
	itemPkgbase      // pkgbase variable
	itemPkgdesc      // pkgdesc variable
	itemArch         // arch variable
	itemURL          // url variable
	itemLicense      // license variable
	itemGroups       // groups variable
	itemDepends      // depends variable
	itemOptdepends   // optdepends variable
	itemMakedepends  // makedepends variable
	itemCheckdepends // checkdepends variable
	itemProvides     // provides variable
	itemConflicts    // conflicts variable
	itemReplaces     // replaces variable
	itemBackup       // backup variable
	itemOptions      // options variable
	itemInstall      // install variable
	itemChangelog    // changelog variable
	itemSource       // source variable
	itemNoextract    // noextract variable
	itemMd5sums      // md5sums variable
	itemSha1sums     // sha1sums variable
	itemSha224sums   // sha224sums variable
	itemSha256sums   // sha256sums variable
	itemSha384sums   // sha384sums variable
	itemSha512sums   // sha512sums variable
	itemValidpgpkeys // validpgpkeys variable
)

// PKGBUILD variables
var variables = map[string]itemType{
	"pkgname":      itemPkgname,
	"pkgver":       itemPkgver,
	"pkgrel":       itemPkgrel,
	"pkgdir":       itemPkgdir,
	"epoch":        itemEpoch,
	"pkgbase":      itemPkgbase,
	"pkgdesc":      itemPkgdesc,
	"arch":         itemArch,
	"url":          itemURL,
	"license":      itemLicense,
	"groups":       itemGroups,
	"depends":      itemDepends,
	"optdepends":   itemOptdepends,
	"makedepends":  itemMakedepends,
	"checkdepends": itemCheckdepends,
	"provides":     itemProvides,
	"conflicts":    itemConflicts,
	"replaces":     itemReplaces,
	"backup":       itemBackup,
	"options":      itemOptions,
	"install":      itemInstall,
	"changelog":    itemChangelog,
	"source":       itemSource,
	"noextract":    itemNoextract,
	"md5sums":      itemMd5sums,
	"sha1sums":     itemSha1sums,
	"sha224sums":   itemSha224sums,
	"sha256sums":   itemSha256sums,
	"sha384sums":   itemSha384sums,
	"sha512sums":   itemSha512sums,
	"validpgpkeys": itemValidpgpkeys,
}

const eof = -1

// stateFn represents the state of the scanner as a function that returns the next state
type stateFn func(*lexer) stateFn

// lexer holds the state of the scanner
type lexer struct {
	input   string
	state   stateFn
	pos     pos
	start   pos
	width   pos
	lastPos pos
	items   chan item // channel of scanned items
}

// next returns the next rune in the input
func (l *lexer) next() rune {
	if int(l.pos) >= len(l.input) {
		l.width = 0
		return eof
	}
	r, w := utf8.DecodeRuneInString(l.input[l.pos:])
	l.width = pos(w)
	l.pos += l.width
	return r
}

// peek returns but does not consume the next rune in the input
func (l *lexer) peek() rune {
	r := l.next()
	l.backup()
	return r
}

// backup steps back one rune. Can only be called once per call of next
func (l *lexer) backup() {
	l.pos -= l.width
}

// emit passes an item back to the client
func (l *lexer) emit(t itemType) {
	l.items <- item{t, l.start, l.input[l.start:l.pos]}
	l.start = l.pos
}

// ignore skips over the pending input before this point
func (l *lexer) ignore() {
	l.start = l.pos
}

// errorf returns an error token and terminates the scan by passing
// back a nil pointer that will be the next state, terminating l.nextItem.
func (l *lexer) errorf(format string, args ...interface{}) stateFn {
	l.items <- item{itemError, l.start, fmt.Sprintf(format, args...)}
	return nil
}

// nextItem returns the next item from the input.
func (l *lexer) nextItem() item {
	item := <-l.items
	l.lastPos = item.pos
	return item
}

func lex(input string) *lexer {
	l := &lexer{
		input: input,
		items: make(chan item),
	}
	go l.run()
	return l
}

func (l *lexer) run() {
	for l.state = lexEnv; l.state != nil; {
		l.state = l.state(l)
	}
}

func lexEnv(l *lexer) stateFn {
	var r rune
	for {
		switch r = l.next(); {
		case r == eof:
			l.emit(itemEOF)
			return nil
		case isAlphaNumericUnderscore(r):
			return lexVariable
		case r == '\n':
			buffer := l.input[l.start:l.pos]
			if buffer == "\n" {
				if l.peek() == '\n' {
					l.next()
					l.emit(itemEndSplit)
				}
				l.ignore()
			}
		case r == '\t':
			l.ignore()
		case r == ' ':
			l.ignore()
		case r == '#':
			return lexComment
		default:
			l.errorf("unable to parse character: %c", r)
		}
	}
}

func lexComment(l *lexer) stateFn {
	for {
		switch l.next() {
		case '\n':
			l.ignore()
			return lexEnv
		case eof:
			l.emit(itemEOF)
			return nil
		}
	}
}

func lexVariable(l *lexer) stateFn {
	for {
		switch r := l.next(); {
		case isAlphaNumericUnderscore(r):
			// absorb
		case r == ' ' && l.peek() == '=':
			l.backup()
			variable := l.input[l.start:l.pos]

			// strip arch from source_arch like constructs
			witharch := strings.SplitN(variable, "_", 2)
			if len(witharch) == 2 {
				variable = witharch[0]
			}

			if _, ok := variables[variable]; ok {
				l.emit(variables[variable])
				// TODO to cut off ' = '
				l.next()
				l.next()
				l.next()
				l.ignore()
				return lexValue
			}
			return l.errorf("invalid variable: %s", variable)
		default:
			pattern := l.input[l.start:l.pos]
			return l.errorf("invalid pattern: %s", pattern)
		}
	}
}

func lexValue(l *lexer) stateFn {
	for {
		switch l.next() {
		case '\n':
			l.backup()
			l.emit(itemValue)
			return lexEnv
		}
	}
}

// isAlphaNumericUnderscore reports whether r is an alphabetic, digit, or underscore.
func isAlphaNumericUnderscore(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
