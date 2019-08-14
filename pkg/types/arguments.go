package types

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Arguments holds command line arguments in a way we can interact with programmatically but
// also in a way that can easily be passed to pacman later on.
type Arguments struct {
	Op      string
	Options map[string]string
	Globals map[string]string
	Doubles StringSet // Tracks args passed twice such as -yy and -dd
	Targets []string
}

// MakeArguments creates an empty arguments structure.
func MakeArguments() *Arguments {
	return &Arguments{
		"",
		make(map[string]string),
		make(map[string]string),
		make(StringSet),
		make([]string, 0),
	}
}

// CopyGlobal makes a new set of Arguments as a copy of the input.
func (parser *Arguments) CopyGlobal() (cp *Arguments) {
	cp = MakeArguments()
	for k, v := range parser.Globals {
		cp.Globals[k] = v
	}

	return
}

// Copy copies all arguments into a new structure.
func (parser *Arguments) Copy() (cp *Arguments) {
	cp = MakeArguments()

	cp.Op = parser.Op

	for k, v := range parser.Options {
		cp.Options[k] = v
	}

	for k, v := range parser.Globals {
		cp.Globals[k] = v
	}

	cp.Targets = make([]string, len(parser.Targets))
	copy(cp.Targets, parser.Targets)

	for k, v := range parser.Doubles {
		cp.Doubles[k] = v
	}

	return
}

// DelArg deletes an argument from the structure.
func (parser *Arguments) DelArg(options ...string) {
	for _, option := range options {
		delete(parser.Options, option)
		delete(parser.Globals, option)
		delete(parser.Doubles, option)
	}
}

// NeedRoot checks arguments for possible priviledge elevation needs.
func (parser *Arguments) NeedRoot(mode TargetMode) bool {
	if parser.ExistsArg("h", "help") {
		return false
	}

	switch parser.Op {
	case "D", "database":
		if parser.ExistsArg("k", "check") {
			return false
		}
		return true
	case "F", "files":
		if parser.ExistsArg("y", "refresh") {
			return true
		}
		return false
	case "Q", "query":
		if parser.ExistsArg("k", "check") {
			return true
		}
		return false
	case "R", "remove":
		if parser.ExistsArg("p", "print", "print-format") {
			return false
		}
		return true
	case "S", "sync":
		if parser.ExistsArg("y", "refresh") {
			return true
		}
		if parser.ExistsArg("p", "print", "print-format") {
			return false
		}
		if parser.ExistsArg("s", "search") {
			return false
		}
		if parser.ExistsArg("l", "list") {
			return false
		}
		if parser.ExistsArg("g", "groups") {
			return false
		}
		if parser.ExistsArg("i", "info") {
			return false
		}
		if parser.ExistsArg("c", "clean") && mode == AUR {
			return false
		}
		return true
	case "U", "upgrade":
		return true
	default:
		return false
	}
}

func (parser *Arguments) addOP(op string) (err error) {
	if parser.Op != "" {
		err = fmt.Errorf("only one operation may be used at a time")
		return
	}

	parser.Op = op
	return
}

func (parser *Arguments) addParam(option string, arg string) (err error) {
	if !isArg(option) {
		return fmt.Errorf("invalid option '%s'", option)
	}

	if isOp(option) {
		err = parser.addOP(option)
		return
	}

	switch {
	case parser.ExistsArg(option):
		parser.Doubles[option] = struct{}{}
	case isGlobal(option):
		parser.Globals[option] = arg
	default:
		parser.Options[option] = arg
	}

	return
}

// AddArg adds an argument to the Arguments
func (parser *Arguments) AddArg(options ...string) (err error) {
	for _, option := range options {
		err = parser.addParam(option, "")
		if err != nil {
			return
		}
	}

	return
}

// ExistsArg checks for existence of argument
// Multiple args acts as an OR operator
func (parser *Arguments) ExistsArg(options ...string) bool {
	for _, option := range options {
		_, exists := parser.Options[option]
		if exists {
			return true
		}

		_, exists = parser.Globals[option]
		if exists {
			return true
		}
	}
	return false
}

// GetArg checks the existance of an option and returns its value.
func (parser *Arguments) GetArg(options ...string) (arg string, double bool, exists bool) {
	existCount := 0

	for _, option := range options {
		var value string

		value, exists = parser.Options[option]

		if exists {
			arg = value
			existCount++
			_, exists = parser.Doubles[option]

			if exists {
				existCount++
			}

		}

		value, exists = parser.Globals[option]

		if exists {
			arg = value
			existCount++
			_, exists = parser.Doubles[option]

			if exists {
				existCount++
			}

		}
	}

	double = existCount >= 2
	exists = existCount >= 1

	return
}

// AddTarget appends targets to the Arguments.Targets
func (parser *Arguments) AddTarget(targets ...string) {
	parser.Targets = append(parser.Targets, targets...)
}

// ClearTargets removes all targets from the structure.
func (parser *Arguments) ClearTargets() {
	parser.Targets = make([]string, 0)
}

// ExistsDouble checks the existance of a double argument
// Multiple args acts as an OR operator
func (parser *Arguments) ExistsDouble(options ...string) bool {
	for _, option := range options {
		_, exists := parser.Doubles[option]
		if exists {
			return true
		}
	}

	return false
}

// FormatArgs turns Arguments into a slice for usage
func (parser *Arguments) FormatArgs() (args []string) {
	var op string

	if parser.Op != "" {
		op = formatArg(parser.Op)
	}

	args = append(args, op)

	for option, arg := range parser.Options {
		if option == "--" {
			continue
		}

		formattedOption := formatArg(option)
		args = append(args, formattedOption)

		if hasParam(option) {
			args = append(args, arg)
		}

		if parser.ExistsDouble(option) {
			args = append(args, formattedOption)
		}
	}

	return
}

// FormatGlobals returns a string slice with global arguments formated.
func (parser *Arguments) FormatGlobals() (args []string) {
	for option, arg := range parser.Globals {
		formattedOption := formatArg(option)
		args = append(args, formattedOption)

		if hasParam(option) {
			args = append(args, arg)
		}

		if parser.ExistsDouble(option) {
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
	case "batchinstall":
	case "nobatchinstall":
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

// ParseShortOption parses short hand options such as:
// -Syu -b/some/path -
func (parser *Arguments) ParseShortOption(arg string, param string) (usedNext bool, err error) {
	if arg == "-" {
		err = parser.AddArg("-")
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
			err = parser.AddArg(char)

			if err != nil {
				return
			}
		}
	}

	return
}

// ParseLongOption parses full length options such as:
// --sync --refresh --sysupgrade --dbpath /some/path --
func (parser *Arguments) ParseLongOption(arg string, param string) (usedNext bool, err error) {
	if arg == "--" {
		err = parser.AddArg(arg)
		return
	}

	arg = arg[2:]

	switch split := strings.SplitN(arg, "=", 2); {
	case len(split) == 2:
		err = parser.addParam(split[0], split[1])
	case hasParam(arg):
		err = parser.addParam(arg, param)
		usedNext = true
	default:
		err = parser.AddArg(arg)
	}

	return
}

// ParseStdin parses targets from Stdin.
func (parser *Arguments) ParseStdin() error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		parser.AddTarget(scanner.Text())
	}

	return os.Stdin.Close()
}
