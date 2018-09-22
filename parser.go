package main

import (
	"bufio"
	"bytes"
	"fmt"
	"html"
	"os"
	"strconv"
	"strings"
	"unicode"

	rpc "github.com/mikkeloscar/aur"
)

// A basic set implementation for strings.
// This is used a lot so it deserves its own type.
// Other types of sets are used throughout the code but do not have
// their own typedef.
// String sets and <type>sets should be used throughout the code when applicable,
// they are a lot more flexible than slices and provide easy lookup.
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

func (set stringSet) copy() stringSet {
	newSet := make(stringSet)

	for str := range set {
		newSet.set(str)
	}

	return newSet
}

func sliceToStringSet(in []string) stringSet {
	set := make(stringSet)

	for _, v := range in {
		set.set(v)
	}

	return set
}

func makeStringSet(in ...string) stringSet {
	return sliceToStringSet(in)
}

// Parses command line arguments in a way we can interact with programmatically but
// also in a way that can easily be passed to pacman later on.
type arguments struct {
	op      string
	options map[string]string
	globals map[string]string
	doubles stringSet // Tracks args passed twice such as -yy and -dd
	targets []string
}

func makeArguments() *arguments {
	return &arguments{
		"",
		make(map[string]string),
		make(map[string]string),
		make(stringSet),
		make([]string, 0),
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

	cp.targets = make([]string, len(parser.targets))
	copy(cp.targets, parser.targets)

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

	switch parser.op {
	case "D", "database":
		if parser.existsArg("k", "check") {
			return false
		}
		return true
	case "F", "files":
		if parser.existsArg("y", "refresh") {
			return true
		}
		return false
	case "Q", "query":
		if parser.existsArg("k", "check") {
			return true
		}
		return false
	case "R", "remove":
		return true
	case "S", "sync":
		if parser.existsArg("y", "refresh") {
			return true
		}
		if parser.existsArg("p", "print", "print-format") {
			return false
		}
		if parser.existsArg("s", "search") {
			return false
		}
		if parser.existsArg("l", "list") {
			return false
		}
		if parser.existsArg("g", "groups") {
			return false
		}
		if parser.existsArg("i", "info") {
			return false
		}
		if parser.existsArg("c", "clean") && config.mode == modeAUR {
			return false
		}
		return true
	case "U", "upgrade":
		return true
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
	if !isArg(option) {
		return fmt.Errorf("invalid option '%s'", option)
	}

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
	existCount := 0

	for _, option := range options {
		var value string

		value, exists = parser.options[option]

		if exists {
			arg = value
			existCount++
			_, exists = parser.doubles[option]

			if exists {
				existCount++
			}

		}

		value, exists = parser.globals[option]

		if exists {
			arg = value
			existCount++
			_, exists = parser.doubles[option]

			if exists {
				existCount++
			}

		}
	}

	double = existCount >= 2
	exists = existCount >= 1

	return
}

func (parser *arguments) addTarget(targets ...string) {
	parser.targets = append(parser.targets, targets...)
}

func (parser *arguments) clearTargets() {
	parser.targets = make([]string, 0)
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

		formattedOption := formatArg(option)
		args = append(args, formattedOption)

		if hasParam(option) {
			args = append(args, arg)
		}

		if parser.existsDouble(option) {
			args = append(args, formattedOption)
		}
	}

	return
}

func (parser *arguments) formatGlobals() (args []string) {
	for option, arg := range parser.globals {
		formattedOption := formatArg(option)
		args = append(args, formattedOption)

		if hasParam(option) {
			args = append(args, arg)
		}

		if parser.existsDouble(option) {
			args = append(args, formattedOption)
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

func isArg(arg string) bool {
	switch arg {
	case "-", "--":
	case "ask":
	case "D", "database":
	case "Q", "query":
	case "R", "remove":
	case "S", "sync":
	case "T", "deptest":
	case "U", "upgrade":
	case "F", "files":
	case "V", "version":
	case "h", "help":
	case "Y", "yay":
	case "P", "show":
	case "G", "getpkgbuild":
	case "b", "dbpath":
	case "r", "root":
	case "v", "verbose":
	case "arch":
	case "cachedir":
	case "color":
	case "config":
	case "debug":
	case "gpgdir":
	case "hookdir":
	case "logfile":
	case "noconfirm":
	case "confirm":
	case "disable-download-timeout":
	case "sysroot":
	case "d", "nodeps":
	case "assume-installed":
	case "dbonly":
	case "noprogressbar":
	case "noscriptlet":
	case "p", "print":
	case "print-format":
	case "asdeps":
	case "asexplicit":
	case "ignore":
	case "ignoregroup":
	case "needed":
	case "overwrite":
	case "f", "force":
	case "c", "changelog":
	case "deps":
	case "e", "explicit":
	case "g", "groups":
	case "i", "info":
	case "k", "check":
	case "l", "list":
	case "m", "foreign":
	case "n", "native":
	case "o", "owns":
	case "file":
	case "q", "quiet":
	case "s", "search":
	case "t", "unrequired":
	case "u", "upgrades":
	case "cascade":
	case "nosave":
	case "recursive":
	case "unneeded":
	case "clean":
	case "sysupgrade":
	case "w", "downloadonly":
	case "y", "refresh":
	case "x", "regex":
	case "machinereadable":
	//yay options
	case "aururl":
	case "save":
	case "afterclean", "cleanafter":
	case "noafterclean", "nocleanafter":
	case "devel":
	case "nodevel":
	case "timeupdate":
	case "notimeupdate":
	case "topdown":
	case "bottomup":
	case "completioninterval":
	case "sortby":
	case "redownload":
	case "redownloadall":
	case "noredownload":
	case "rebuild":
	case "rebuildall":
	case "rebuildtree":
	case "norebuild":
	case "answerclean":
	case "noanswerclean":
	case "answerdiff":
	case "noanswerdiff":
	case "answeredit":
	case "noansweredit":
	case "answerupgrade":
	case "noanswerupgrade":
	case "gitclone":
	case "nogitclone":
	case "gpgflags":
	case "mflags":
	case "gitflags":
	case "builddir":
	case "editor":
	case "editorflags":
	case "makepkg":
	case "makepkgconf":
	case "nomakepkgconf":
	case "pacman":
	case "tar":
	case "git":
	case "gpg":
	case "requestsplitn":
	case "sudoloop":
	case "nosudoloop":
	case "provides":
	case "noprovides":
	case "pgpfetch":
	case "nopgpfetch":
	case "upgrademenu":
	case "noupgrademenu":
	case "cleanmenu":
	case "nocleanmenu":
	case "diffmenu":
	case "nodiffmenu":
	case "editmenu":
	case "noeditmenu":
	case "useask":
	case "nouseask":
	case "combinedupgrade":
	case "nocombinedupgrade":
	case "a", "aur":
	case "repo":
	case "removemake":
	case "noremovemake":
	case "askremovemake":
	case "complete":
	case "stats":
	case "news":
	case "gendb":
	case "currentconfig":
	default:
		return false
	}

	return true
}

func handleConfig(option, value string) bool {
	switch option {
	case "aururl":
		config.value["aururl"] = value
	case "save":
		config.shouldSaveConfig = true
	case "afterclean", "cleanafter":
		config.boolean["cleanafter"] = true
	case "noafterclean", "nocleanafter":
		config.boolean["cleanafter"] = false
	case "devel":
		config.boolean["devel"] = true
	case "nodevel":
		config.boolean["devel"] = false
	case "timeupdate":
		config.boolean["timeupdate"] = true
	case "notimeupdate":
		config.boolean["timeupdate"] = false
	case "topdown":
		config.value["sortmode"] = "topdown"
	case "bottomup":
		config.value["sortmode"] = "bottomup"
	case "completioninterval":
		n, err := strconv.Atoi(value)
		if err == nil {
			config.num["completioninterval"] = n
		}
	case "sortby":
		config.value["sortby"] = value
	case "noconfirm":
		config.noConfirm = true
	case "config":
		config.value["pacmanconf"] = value
	case "redownload":
		config.value["redownload"] = "yes"
	case "redownloadall":
		config.value["redownload"] = "all"
	case "noredownload":
		config.value["redownload"] = "no"
	case "rebuild":
		config.value["rebuild"] = "yes"
	case "rebuildall":
		config.value["rebuild"] = "all"
	case "rebuildtree":
		config.value["rebuild"] = "tree"
	case "norebuild":
		config.value["rebuild"] = "no"
	case "answerclean":
		config.value["answerclean"] = value
	case "noanswerclean":
		config.value["answerclean"] = ""
	case "answerdiff":
		config.value["answerdiff"] = value
	case "noanswerdiff":
		config.value["answerdiff"] = ""
	case "answeredit":
		config.value["answeredit"] = value
	case "noansweredit":
		config.value["answeredit"] = ""
	case "answerupgrade":
		config.value["answerupgrade"] = value
	case "noanswerupgrade":
		config.value["answerupgrade"] = ""
	case "gitclone":
		config.boolean["gitclone"] = true
	case "nogitclone":
		config.boolean["gitclone"] = false
	case "gpgflags":
		config.value["gpgflags"] = value
	case "mflags":
		config.value["makepkgflags"] = value
	case "gitflags":
		config.value["gitflags"] = value
	case "builddir":
		config.value["builddir"] = value
	case "editor":
		config.value["editor"] = value
	case "editorflags":
		config.value["editorflags"] = value
	case "makepkg":
		config.value["makepkgcommand"] = value
	case "makepkgconf":
		config.value["makepkgconf"] = value
	case "nomakepkgconf":
		config.value["makepkgconf"] = ""
	case "pacman":
		config.value["pacmancommand"] = value
	case "tar":
		config.value["tarcommand"] = value
	case "git":
		config.value["gitcommand"] = value
	case "gpg":
		config.value["gpgcommand"] = value
	case "requestsplitn":
		n, err := strconv.Atoi(value)
		if err == nil && n > 0 {
			config.num["requestsplitn"] = n
		}
	case "sudoloop":
		config.boolean["sudoloop"] = true
	case "nosudoloop":
		config.boolean["sudoloop"] = false
	case "provides":
		config.boolean["provides"] = true
	case "noprovides":
		config.boolean["provides"] = false
	case "pgpfetch":
		config.boolean["pgpfetch"] = true
	case "nopgpfetch":
		config.boolean["pgpfetch"] = false
	case "upgrademenu":
		config.boolean["upgrademenu"] = true
	case "noupgrademenu":
		config.boolean["upgrademenu"] = false
	case "cleanmenu":
		config.boolean["cleanmenu"] = true
	case "nocleanmenu":
		config.boolean["cleanmenu"] = false
	case "diffmenu":
		config.boolean["diffmenu"] = true
	case "nodiffmenu":
		config.boolean["diffmenu"] = false
	case "editmenu":
		config.boolean["editmenu"] = true
	case "noeditmenu":
		config.boolean["editmenu"] = false
	case "useask":
		config.boolean["useask"] = true
	case "nouseask":
		config.boolean["useask"] = false
	case "combinedupgrade":
		config.boolean["combinedupgrade"] = true
	case "nocombinedupgrade":
		config.boolean["combinedupgrade"] = false
	case "a", "aur":
		config.mode = modeAUR
	case "repo":
		config.mode = modeRepo
	case "removemake":
		config.value["removemake"] = "yes"
	case "noremovemake":
		config.value["removemake"] = "no"
	case "askremovemake":
		config.value["removemake"] = "ask"
	default:
		return false
	}

	return true
}

func isOp(op string) bool {
	switch op {
	case "V", "version":
	case "D", "database":
	case "F", "files":
	case "Q", "query":
	case "R", "remove":
	case "S", "sync":
	case "T", "deptest":
	case "U", "upgrade":
	// yay specific
	case "Y", "yay":
	case "P", "show":
	case "G", "getpkgbuild":
	default:
		return false
	}

	return true
}

func isGlobal(op string) bool {
	switch op {
	case "b", "dbpath":
	case "r", "root":
	case "v", "verbose":
	case "arch":
	case "cachedir":
	case "color":
	case "config":
	case "debug":
	case "gpgdir":
	case "hookdir":
	case "logfile":
	case "noconfirm":
	case "confirm":
	default:
		return false
	}

	return true
}

func hasParam(arg string) bool {
	switch arg {
	case "dbpath", "b":
	case "root", "r":
	case "sysroot":
	case "config":
	case "ignore":
	case "assume-installed":
	case "overwrite":
	case "ask":
	case "cachedir":
	case "hookdir":
	case "logfile":
	case "ignoregroup":
	case "arch":
	case "print-format":
	case "gpgdir":
	case "color":
	//yay params
	case "aururl":
	case "mflags":
	case "gpgflags":
	case "gitflags":
	case "builddir":
	case "editor":
	case "editorflags":
	case "makepkg":
	case "makepkgconf":
	case "pacman":
	case "tar":
	case "git":
	case "gpg":
	case "requestsplitn":
	case "answerclean":
	case "answerdiff":
	case "answeredit":
	case "answerupgrade":
	case "completioninterval":
	case "sortby":
	default:
		return false
	}

	return true
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
			if k < len(arg)-1 {
				err = parser.addParam(char, arg[k+1:])
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

	split := strings.SplitN(arg, "=", 2)
	if len(split) == 2 {
		err = parser.addParam(split[0], split[1])
	} else if hasParam(arg) {
		err = parser.addParam(arg, param)
		usedNext = true
	} else {
		err = parser.addArg(arg)
	}

	return
}

func (parser *arguments) parseStdin() error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		parser.addTarget(scanner.Text())
	}

	return os.Stdin.Close()
}

func (parser *arguments) parseCommandLine() (err error) {
	args := os.Args[1:]
	usedNext := false

	if len(args) < 1 {
		parser.parseShortOption("-Syu", "")
	} else {
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
	}

	if parser.op == "" {
		parser.op = "Y"
	}

	if parser.existsArg("-") {
		var file *os.File
		err = parser.parseStdin()
		parser.delArg("-")

		if err != nil {
			return
		}

		file, err = os.Open("/dev/tty")

		if err != nil {
			return
		}

		os.Stdin = file
	}

	cmdArgs.extractYayOptions()
	return
}

func (parser *arguments) extractYayOptions() {

	for option, value := range parser.options {
		if handleConfig(option, value) {
			parser.delArg(option)
		}
	}

	for option, value := range parser.globals {
		if handleConfig(option, value) {
			parser.delArg(option)
		}
	}

	rpc.AURURL = strings.TrimRight(config.value["aururl"], "/") + "/rpc.php?"
	config.value["aururl"] = strings.TrimRight(config.value["aururl"], "/")
}

//parses input for number menus split by spaces or commas
//supports individual selection: 1 2 3 4
//supports range selections: 1-4 10-20
//supports negation: ^1 ^1-4
//
//include and excule holds numbers that should be added and should not be added
//respectively. other holds anything that can't be parsed as an int. This is
//intended to allow words inside of number menus. e.g. 'all' 'none' 'abort'
//of course the implementation is up to the caller, this function mearley parses
//the input and organizes it
func parseNumberMenu(input string) (intRanges, intRanges, stringSet, stringSet) {
	include := make(intRanges, 0)
	exclude := make(intRanges, 0)
	otherInclude := make(stringSet)
	otherExclude := make(stringSet)

	words := strings.FieldsFunc(input, func(c rune) bool {
		return unicode.IsSpace(c) || c == ','
	})

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

// Crude html parsing, good enough for the arch news
// This is only displayed in the terminal so there should be no security
// concerns
func parseNews(str string) string {
	var buffer bytes.Buffer
	var tagBuffer bytes.Buffer
	var escapeBuffer bytes.Buffer
	inTag := false
	inEscape := false

	for _, char := range str {
		if inTag {
			if char == '>' {
				inTag = false
				switch tagBuffer.String() {
				case "code":
					buffer.WriteString(cyanCode)
				case "/code":
					buffer.WriteString(resetCode)
				case "/p":
					buffer.WriteRune('\n')
				}

				continue
			}

			tagBuffer.WriteRune(char)
			continue
		}

		if inEscape {
			if char == ';' {
				inEscape = false
				escapeBuffer.WriteRune(char)
				s := html.UnescapeString(escapeBuffer.String())
				buffer.WriteString(s)
				continue
			}

			escapeBuffer.WriteRune(char)
			continue
		}

		if char == '<' {
			inTag = true
			tagBuffer.Reset()
			continue
		}

		if char == '&' {
			inEscape = true
			escapeBuffer.Reset()
			escapeBuffer.WriteRune(char)
			continue
		}

		buffer.WriteRune(char)
	}

	buffer.WriteString(resetCode)
	return buffer.String()
}
