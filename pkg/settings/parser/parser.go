package parser

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/leonelquinteros/gotext"
)

type Option struct {
	Global bool
	Args   []string
}

func (o *Option) Add(args ...string) {
	if o.Args == nil {
		o.Args = args
		return
	}

	o.Args = append(o.Args, args...)
}

func (o *Option) First() string {
	if o.Args == nil || len(o.Args) == 0 {
		return ""
	}

	return o.Args[0]
}

func (o *Option) Set(arg string) {
	o.Args = []string{arg}
}

func (o *Option) String() string {
	return fmt.Sprintf("Global:%v Args:%v", o.Global, o.Args)
}

// Arguments Parses command line arguments in a way we can interact with programmatically but
// also in a way that can easily be passed to pacman later on.
type Arguments struct {
	Op      string
	Options map[string]*Option
	Targets []string
}

func (a *Arguments) String() string {
	return fmt.Sprintf("Op:%v Options:%+v Targets: %v", a.Op, a.Options, a.Targets)
}

func (a *Arguments) CreateOrAppendOption(option string, values ...string) {
	if a.Options[option] == nil {
		a.Options[option] = &Option{
			Args: values,
		}
	} else {
		a.Options[option].Add(values...)
	}
}

func MakeArguments() *Arguments {
	return &Arguments{
		"",
		make(map[string]*Option),
		make([]string, 0),
	}
}

func (a *Arguments) CopyGlobal() *Arguments {
	cp := MakeArguments()

	for k, v := range a.Options {
		if v.Global {
			cp.Options[k] = v
		}
	}

	return cp
}

func (a *Arguments) Copy() (cp *Arguments) {
	cp = MakeArguments()

	cp.Op = a.Op

	for k, v := range a.Options {
		cp.Options[k] = v
	}

	cp.Targets = make([]string, len(a.Targets))
	copy(cp.Targets, a.Targets)

	return
}

func (a *Arguments) DelArg(options ...string) {
	for _, option := range options {
		delete(a.Options, option)
	}
}

func (a *Arguments) NeedRoot(mode TargetMode) bool {
	if a.ExistsArg("h", "help") {
		return false
	}

	switch a.Op {
	case "D", "database":
		if a.ExistsArg("k", "check") {
			return false
		}

		return true
	case "F", "files":
		if a.ExistsArg("y", "refresh") {
			return true
		}

		return false
	case "Q", "query":
		if a.ExistsArg("k", "check") {
			return true
		}

		return false
	case "R", "remove":
		if a.ExistsArg("p", "print", "print-format") {
			return false
		}

		return true
	case "S", "sync":
		switch {
		case a.ExistsArg("y", "refresh"):
			return true
		case a.ExistsArg("p", "print", "print-format"):
			return false
		case a.ExistsArg("s", "search"):
			return false
		case a.ExistsArg("l", "list"):
			return false
		case a.ExistsArg("g", "groups"):
			return false
		case a.ExistsArg("i", "info"):
			return false
		case a.ExistsArg("c", "clean") && mode == ModeAUR:
			return false
		}

		return true
	case "U", "upgrade":
		return true
	default:
		return false
	}
}

func (a *Arguments) addOP(op string) error {
	if a.Op != "" {
		return errors.New(gotext.Get("only one operation may be used at a time"))
	}

	a.Op = op

	return nil
}

func (a *Arguments) addParam(option, arg string) error {
	if !isArg(option) {
		return errors.New(gotext.Get("invalid option '%s'", option))
	}

	if isOp(option) {
		return a.addOP(option)
	}

	a.CreateOrAppendOption(option, strings.Split(arg, ",")...)

	if isGlobal(option) {
		a.Options[option].Global = true
	}

	return nil
}

func (a *Arguments) AddArg(options ...string) error {
	for _, option := range options {
		err := a.addParam(option, "")
		if err != nil {
			return err
		}
	}

	return nil
}

// Multiple args acts as an OR operator.
func (a *Arguments) ExistsArg(options ...string) bool {
	for _, option := range options {
		if _, exists := a.Options[option]; exists {
			return true
		}
	}

	return false
}

func (a *Arguments) GetArg(options ...string) (arg string, double, exists bool) {
	for _, option := range options {
		value, exists := a.Options[option]
		if exists {
			return value.First(), len(value.Args) >= 2, len(value.Args) >= 1
		}
	}

	return arg, false, false
}

func (a *Arguments) GetArgs(option string) (args []string) {
	value, exists := a.Options[option]
	if exists {
		return value.Args
	}

	return nil
}

func (a *Arguments) AddTarget(targets ...string) {
	a.Targets = append(a.Targets, targets...)
}

func (a *Arguments) ClearTargets() {
	a.Targets = make([]string, 0)
}

// Multiple args acts as an OR operator.
func (a *Arguments) ExistsDouble(options ...string) bool {
	for _, option := range options {
		if value, exists := a.Options[option]; exists {
			return len(value.Args) >= 2
		}
	}

	return false
}

func (a *Arguments) FormatArgs() (args []string) {
	if a.Op != "" {
		args = append(args, formatArg(a.Op))
	}

	for option, arg := range a.Options {
		if arg.Global || option == "--" {
			continue
		}

		formattedOption := formatArg(option)
		for _, value := range arg.Args {
			args = append(args, formattedOption)
			if hasParam(option) {
				args = append(args, value)
			}
		}
	}

	return args
}

func (a *Arguments) FormatGlobals() (args []string) {
	for option, arg := range a.Options {
		if !arg.Global {
			continue
		}

		formattedOption := formatArg(option)

		for _, value := range arg.Args {
			args = append(args, formattedOption)
			if hasParam(option) {
				args = append(args, value)
			}
		}
	}

	return args
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
	case "W", "web":
	case "P", "show":
	case "B", "build":
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
	case "numberupgrades":
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
	case "disable-sandbox":
	// yay options
	case "aururl":
	case "aurrpcurl":
	case "save":
	case "afterclean", "cleanafter":
	case "keepsrc":
	case "devel":
	case "timeupdate":
	case "topdown":
	case "bottomup":
	case "completioninterval":
	case "sortby":
	case "searchby":
	case "redownload":
	case "redownloadall":
	case "noredownload":
	case "rebuild":
	case "rebuildall":
	case "rebuildtree":
	case "norebuild":
	case "batchinstall":
	case "answerclean":
	case "noanswerclean":
	case "answerdiff":
	case "noanswerdiff":
	case "answeredit":
	case "noansweredit":
	case "answerupgrade":
	case "noanswerupgrade":
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
	case "git":
	case "gpg":
	case "sudo":
	case "sudoflags":
	case "requestsplitn":
	case "sudoloop":
	case "provides":
	case "pgpfetch":
	case "cleanmenu":
	case "diffmenu":
	case "editmenu":
	case "useask":
	case "combinedupgrade":
	case "a", "aur":
	case "N", "repo":
	case "removemake":
	case "noremovemake":
	case "askremovemake":
	case "askyesremovemake":
	case "complete":
	case "stats":
	case "news":
	case "gendb":
	case "currentconfig":
	case "defaultconfig":
	case "singlelineresults":
	case "doublelineresults":
	case "separatesources":
	case "showpackageurls":
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
	case "W", "web":
	case "B", "build":
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
	// yay params
	case "aururl":
	case "aurrpcurl":
	case "mflags":
	case "gpgflags":
	case "gitflags":
	case "builddir":
	case "editor":
	case "editorflags":
	case "makepkg":
	case "makepkgconf":
	case "pacman":
	case "git":
	case "gpg":
	case "sudo":
	case "sudoflags":
	case "requestsplitn":
	case "answerclean":
	case "answerdiff":
	case "answeredit":
	case "answerupgrade":
	case "completioninterval":
	case "sortby":
	case "searchby":
	default:
		return false
	}

	return true
}

// Parses short hand options such as:
// -Syu -b/some/path -.
func (a *Arguments) parseShortOption(arg, param string) (usedNext bool, err error) {
	if arg == "-" {
		err = a.AddArg("-")
		return
	}

	arg = arg[1:]

	for k, _char := range arg {
		char := string(_char)

		if hasParam(char) {
			if k < len(arg)-1 {
				err = a.addParam(char, arg[k+1:])
			} else {
				usedNext = true
				err = a.addParam(char, param)
			}

			break
		} else {
			err = a.AddArg(char)

			if err != nil {
				return
			}
		}
	}

	return
}

// Parses full length options such as:
// --sync --refresh --sysupgrade --dbpath /some/path --.
func (a *Arguments) parseLongOption(arg, param string) (usedNext bool, err error) {
	if arg == "--" {
		err = a.AddArg(arg)
		return
	}

	arg = arg[2:]

	switch split := strings.SplitN(arg, "=", 2); {
	case len(split) == 2:
		err = a.addParam(split[0], split[1])
	case hasParam(arg):
		err = a.addParam(arg, param)
		usedNext = true
	default:
		err = a.AddArg(arg)
	}

	return
}

func (a *Arguments) parseStdin() error {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return err
	}

	// Ensure data is piped
	if (fi.Mode() & os.ModeCharDevice) != 0 {
		return errors.New(gotext.Get("argument '-' specified without input on stdin"))
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		a.AddTarget(scanner.Text())
	}

	return os.Stdin.Close()
}

func (a *Arguments) Parse() error {
	args := os.Args[1:]
	usedNext := false

	for k, arg := range args {
		var nextArg string

		if usedNext {
			usedNext = false
			continue
		}

		if k+1 < len(args) {
			nextArg = args[k+1]
		}

		var err error
		switch {
		case a.ExistsArg("--"):
			a.AddTarget(arg)
		case strings.HasPrefix(arg, "--"):
			usedNext, err = a.parseLongOption(arg, nextArg)
		case strings.HasPrefix(arg, "-"):
			usedNext, err = a.parseShortOption(arg, nextArg)
		default:
			a.AddTarget(arg)
		}

		if err != nil {
			return err
		}
	}

	if a.Op == "" {
		if len(a.Targets) > 0 {
			a.Op = "Y"
		} else {
			if _, err := a.parseShortOption("-Syu", ""); err != nil {
				return err
			}
		}
	}

	if a.ExistsArg("-") {
		if err := a.parseStdin(); err != nil {
			return err
		}

		a.DelArg("-")

		file, err := os.Open("/dev/tty")
		if err != nil {
			return err
		}

		os.Stdin = file
	}

	return nil
}
