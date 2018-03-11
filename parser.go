package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// A basic set implementation for strings.
// This is used a lot so it deserves its own type.
// Other types of sets are used throughout the code but do not have
// their own typedef.
// String sets and <type>sets should be used throughout the code when applicable,
// they are a lot more flexable than slices and provide easy lookup.
type stringSet map[string]struct{}

func (set stringSet) set(v string) {
	set[v] = struct{}{}
}

func (set stringSet) get(v string) bool {
	_, exists := set[v]
	return exists
}

func (set stringSet) remove(v string) {
	delete(set, v)
}

func (set stringSet) toSlice() []string {
	slice := make([]string, 0, len(set))

	for v := range set {
		slice = append(slice, v)
	}

	return slice
}

func SliceToStringSet(in []string) stringSet {
	set := make(stringSet)

	for _, v := range in {
		set.set(v)
	}

	return set
}

func makeStringSet(in ...string) stringSet {
	return SliceToStringSet(in)
}

// Parses command line arguments in a way we can interact with programmatically but
// also in a way that can easily be passed to pacman later on.
type arguments struct {
	op      string
	options map[string]string
	globals map[string]string
	doubles stringSet // Tracks args passed twice such as -yy and -dd
	targets stringSet
}

func makeArguments() *arguments {
	return &arguments{
		"",
		make(map[string]string),
		make(map[string]string),
		make(stringSet),
		make(stringSet),
	}
}

func (parser *arguments) copy() (cp *arguments) {
	cp = makeArguments()

	cp.op = parser.op

	for k, v := range parser.options {
		cp.options[k] = v
	}

	for k, v := range parser.globals {
		cp.globals[k] = v
	}

	for k, v := range parser.targets {
		cp.targets[k] = v
	}

	for k, v := range parser.doubles {
		cp.doubles[k] = v
	}

	return
}

func (parser *arguments) delArg(options ...string) {
	for _, option := range options {
		delete(parser.options, option)
		delete(parser.globals, option)
		delete(parser.doubles, option)
	}
}

func (parser *arguments) needRoot() bool {
	if parser.existsArg("h", "help") {
		return false
	}

	if parser.existsArg("p", "print") {
		return false
	}

	switch parser.op {
	case "V", "version":
		return false
	case "D", "database":
		return true
	case "F", "files":
		if parser.existsArg("y", "refresh") {
			return true
		}
		return false
	case "Q", "query":
		return false
	case "R", "remove":
		return true
	case "S", "sync":
		if parser.existsArg("y", "refresh") {
			return true
		}
		if parser.existsArg("u", "sysupgrade") {
			return true
		}
		if parser.existsArg("s", "search") {
			return false
		}
		if parser.existsArg("l", "list") {
			return false
		}
		if parser.existsArg("i", "info") {
			return false
		}
		return true
	case "T", "deptest":
		return false
	case "U", "upgrade":
		return true

	// yay specific
	case "Y", "yay":
		return false
	case "P", "print":
		return false
	case "G", "getpkgbuild":
		return false
	default:
		return false
	}
}

func (parser *arguments) addOP(op string) (err error) {
	if parser.op != "" {
		err = fmt.Errorf("only one operation may be used at a time")
		return
	}

	parser.op = op
	return
}

func (parser *arguments) addParam(option string, arg string) (err error) {
	if isOp(option) {
		err = parser.addOP(option)
		return
	}

	if parser.existsArg(option) {
		parser.doubles[option] = struct{}{}
	} else if isGlobal(option) {
		parser.globals[option] = arg
	} else {
		parser.options[option] = arg
	}

	return
}

func (parser *arguments) addArg(options ...string) (err error) {
	for _, option := range options {
		err = parser.addParam(option, "")
		if err != nil {
			return
		}
	}

	return
}

// Multiple args acts as an OR operator
func (parser *arguments) existsArg(options ...string) bool {
	for _, option := range options {
		_, exists := parser.options[option]
		if exists {
			return true
		}

		_, exists = parser.globals[option]
		if exists {
			return true
		}
	}
	return false
}

func (parser *arguments) getArg(options ...string) (arg string, double bool, exists bool) {
	for _, option := range options {
		arg, exists = parser.options[option]

		if exists {
			_, double = parser.doubles[option]
			return
		}

		arg, exists = parser.globals[option]

		if exists {
			_, double = parser.doubles[option]
			return
		}
	}

	return
}

func (parser *arguments) addTarget(targets ...string) {
	for _, target := range targets {
		parser.targets[target] = struct{}{}
	}
}

func (parser *arguments) delTarget(targets ...string) {
	for _, target := range targets {
		delete(parser.targets, target)
	}
}

// Multiple args acts as an OR operator
func (parser *arguments) existsDouble(options ...string) bool {
	for _, option := range options {
		_, exists := parser.doubles[option]
		if exists {
			return true
		}
	}

	return false
}

func (parser *arguments) formatTargets() (args []string) {
	for target := range parser.targets {
		args = append(args, target)
	}

	return
}

func (parser *arguments) formatArgs() (args []string) {
	var op string

	if parser.op != "" {
		op = formatArg(parser.op)
	}

	args = append(args, op)

	for option, arg := range parser.options {
		if option == "--" {
			continue
		}

		formatedOption := formatArg(option)
		args = append(args, formatedOption)

		if hasParam(option) {
			args = append(args, arg)
		}

		if parser.existsDouble(option) {
			args = append(args, formatedOption)
		}
	}

	return
}

func (parser *arguments) formatGlobals() (args []string) {
	for option, arg := range parser.globals {
		formatedOption := formatArg(option)
		args = append(args, formatedOption)

		if hasParam(option) {
			args = append(args, arg)
		}

		if parser.existsDouble(option) {
			args = append(args, formatedOption)
		}
	}

	return

}

func formatArg(arg string) string {
	if len(arg) > 1 {
		arg = "--" + arg
	} else {
		arg = "-" + arg
	}

	return arg
}

func isOp(op string) bool {
	switch op {
	case "V", "version":
		return true
	case "D", "database":
		return true
	case "F", "files":
		return true
	case "Q", "query":
		return true
	case "R", "remove":
		return true
	case "S", "sync":
		return true
	case "T", "deptest":
		return true
	case "U", "upgrade":
		return true

		// yay specific
	case "Y", "yay":
		return true
	case "P", "print":
		return true
	case "G", "getpkgbuild":
		return true
	default:
		return false
	}
}

func isGlobal(op string) bool {
	switch op {
	case "b", "dbpath":
		return true
	case "r", "root":
		return true
	case "v", "verbose":
		return true
	case "arch":
		return true
	case "cachedir":
		return true
	case "color":
		return true
	case "config":
		return true
	case "debug":
		return true
	case "gpgdir":
		return true
	case "hookdir":
		return true
	case "logfile":
		return true
	case "noconfirm":
		return true
	case "confirm":
		return true
	default:
		return false
	}
}

func hasParam(arg string) bool {
	switch arg {
	case "dbpath", "b":
		return true
	case "root", "r":
		return true
	case "sysroot":
		return true
	case "config":
		return true
	case "ignore":
		return true
	case "assume-installed":
		return true
	case "overwrite":
		return true
	case "ask":
		return true
	case "cachedir":
		return true
	case "hookdir":
		return true
	case "logfile":
		return true
	case "ignoregroup":
		return true
	case "arch":
		return true
	case "print-format":
		return true
	case "gpgdir":
		return true
	case "color":
		return true

	//yay params
	case "mflags":
		return true
	case "builddir":
		return true
	case "editor":
		return true
	case "makepkg":
		return true
	case "pacman":
		return true
	case "tar":
		return true
	case "git":
		return true
	case "requestsplitn":
		return true
	default:
		return false
	}
}

// Parses short hand options such as:
// -Syu -b/some/path -
func (parser *arguments) parseShortOption(arg string, param string) (usedNext bool, err error) {
	if arg == "-" {
		err = parser.addArg("-")
		return
	}

	arg = arg[1:]

	for k, _char := range arg {
		char := string(_char)

		if hasParam(char) {
			if k < len(arg)-2 {
				err = parser.addParam(char, arg[k+2:])
			} else {
				usedNext = true
				err = parser.addParam(char, param)
			}

			break
		} else {
			err = parser.addArg(char)

			if err != nil {
				return
			}
		}
	}

	return
}

// Parses full length options such as:
// --sync --refresh --sysupgrade --dbpath /some/path --
func (parser *arguments) parseLongOption(arg string, param string) (usedNext bool, err error) {
	if arg == "--" {
		err = parser.addArg(arg)
		return
	}

	arg = arg[2:]

	if hasParam(arg) {
		err = parser.addParam(arg, param)
		usedNext = true
	} else {
		err = parser.addArg(arg)
	}

	return
}

func (parser *arguments) parseStdin() (err error) {
	for {
		var target string
		_, err = fmt.Scan(&target)

		if err != nil {
			if err == io.EOF {
				err = nil
			}

			return
		}

		parser.addTarget(target)
	}

	return
}

func (parser *arguments) parseCommandLine() (err error) {
	args := os.Args[1:]
	usedNext := false

	if len(args) < 1 {
		err = fmt.Errorf("no operation specified (use -h for help)")
		return
	}

	for k, arg := range args {
		var nextArg string

		if usedNext {
			usedNext = false
			continue
		}

		if k+1 < len(args) {
			nextArg = args[k+1]
		}

		if parser.existsArg("--") {
			parser.addTarget(arg)
		} else if strings.HasPrefix(arg, "--") {
			usedNext, err = parser.parseLongOption(arg, nextArg)
		} else if strings.HasPrefix(arg, "-") {
			usedNext, err = parser.parseShortOption(arg, nextArg)
		} else {
			parser.addTarget(arg)
		}

		if err != nil {
			return
		}
	}

	if parser.op == "" {
		parser.op = "Y"
	}

	if cmdArgs.existsArg("-") {
		err = cmdArgs.parseStdin()

		if err != nil {
			return
		}
	}

	return
}

type intRange struct {
	min int
	max int
}

func makeIntRange(min, max int) intRange {
	return intRange{
		min,
		max,
	}
}

func (r intRange) get(n int) bool {
	return n >= r.min && n <= r.max
}

type intRanges []intRange

func (rs intRanges) get(n int) bool {
	for _, r := range rs {
		if r.get(n) {
			return true
		}
	}

	return false
}

//parses input for number menus
//supports individual selection: 1 2 3 4
//supports range selections: 1-4 10-20
//supports negation: ^1 ^1-4
//
//include and excule holds numbers that should be added and should not be added
//respectively. other holds anythign that can't be parsed as an int. This is
//intended to allow words inside of number menus. e.g. 'all' 'none' 'abort'
//of course the implementation is up to the caller, this function mearley parses
//the input and organizes it
func parseNumberMenu(input string) (intRanges, intRanges, stringSet, stringSet) {
	include := make(intRanges, 0, 0)
	exclude := make(intRanges, 0, 0)
	otherInclude := make(stringSet)
	otherExclude := make(stringSet)

	words := strings.Fields(input)

	for _, word := range words {
		var num1 int
		var num2 int
		var err error
		invert := false
		other := otherInclude

		if word[0] == '^' {
			invert = true
			other = otherExclude
			word = word[1:]
		}

		ranges := strings.SplitN(word, "-", 2)

		num1, err = strconv.Atoi(ranges[0])
		if err != nil {
			other.set(strings.ToLower(word))
			continue
		}

		if len(ranges) == 2 {
			num2, err = strconv.Atoi(ranges[1])
			if err != nil {
				other.set(strings.ToLower(word))
				continue
			}
		} else {
			num2 = num1
		}

		mi := min(num1, num2)
		ma := max(num1, num2)

		if !invert {
			include = append(include, makeIntRange(mi, ma))
		} else {
			exclude = append(exclude, makeIntRange(mi, ma))
		}
	}

	return include, exclude, otherInclude, otherExclude
}
