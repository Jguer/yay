package settings

import (
	"os"
	"strings"
)

type Arg struct {
	Arg   string
	Value string
}

type Args struct {
	Op      string
	Args    []Arg
	Targets []string
}

func (arg *Arg) String() string {
	if len(arg.Arg) == 1 {
		if arg.Value == "" {
			return "-" + arg.Arg
		} else {
			return "-" + arg.Arg + arg.Value
		}
	} else {
		if arg.Value == "" {
			return "--" + arg.Arg
		} else {
			return "--" + arg.Arg + "=" + arg.Value
		}
	}
}

func (a *Args) Format() []string {
	ret := []string{"-" + a.Op}

	for _, arg := range a.Args {
		ret = append(ret, arg.String())
	}

	ret = append(ret, "--")
	ret = append(ret, a.Targets...)
	return ret
}

func (a *Args) AddParam(_arg string, value string) {
	arg := Arg{_arg, value}
	a.Args = append(a.Args, arg)
}

func (a *Args) Add(args ...string) {
	for _, _arg := range args {
		arg := Arg{Arg: _arg}
		a.Args = append(a.Args, arg)
	}
}

func (a *Args) AddTarget(args ...string) {
	a.Targets = append(a.Targets, args...)
}

func (a *Args) Del(args ...string) {
	for _, arg := range args {
		a.del(arg)
	}
}

func (a *Args) del(arg string) {
	for i, v := range a.Args {
		if v.Arg == arg {
			a.Args[i] = a.Args[len(a.Args)-1]
			a.Args = a.Args[:len(a.Args)-2]
		}
	}
}

func addParam(config *YayConfig, option string, arg string) (err error) {
	return config.setFlag(option, arg)
}

func addArg(config *YayConfig, options ...string) error {
	for _, option := range options {
		err := addParam(config, option, "")
		if err != nil {
			return err
		}
	}
	return nil
}

// Parses short hand options such as:
// -Syu -b/some/path -
func parseShortOption(config *YayConfig, arg, param string) (usedNext bool, err error) {
	if arg == "-" {
		err = addArg(config, "-")
		return
	}

	arg = arg[1:]

	for k, _char := range arg {
		char := string(_char)

		if config.hasParam(char) {
			if k < len(arg)-1 {
				err = addParam(config, char, arg[k+1:])
			} else {
				usedNext = true
				err = addParam(config, char, param)
			}

			break
		} else {
			err = addArg(config, char)

			if err != nil {
				return
			}
		}
	}

	return
}

// Parses full length options such as:
// --sync --refresh --sysupgrade --dbpath /some/path --
func parseLongOption(config *YayConfig, arg, param string) (usedNext bool, err error) {
	if arg == "--" {
		err = addArg(config, arg)
		return
	}

	arg = arg[2:]

	switch split := strings.SplitN(arg, "=", 2); {
	case len(split) == 2:
		err = addParam(config, split[0], split[1])
	case config.hasParam(arg):
		err = addParam(config, arg, param)
		usedNext = true
	default:
		err = addArg(config, arg)
	}

	return
}

func ParseCommandLine(config *YayConfig) error {
	args := os.Args[1:]
	usedNext := false

	if len(args) < 1 {
		if _, err := parseShortOption(config, "-Syu", ""); err != nil {
			return err
		}
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

			var err error
			switch {
			case config.EndOfArgs:
				config.AddTarget(arg)
			case strings.HasPrefix(arg, "--"):
				usedNext, err = parseLongOption(config, arg, nextArg)
			case strings.HasPrefix(arg, "-"):
				usedNext, err = parseShortOption(config, arg, nextArg)
			default:
				config.AddTarget(arg)
			}

			if err != nil {
				return err
			}
		}
	}

	if config.Op == "" {
		config.Op = "Y"
	}

	if config.Stdin {
		if err := config.ParseStdin(); err != nil {
			return err
		}

		file, err := os.Open("/dev/tty")
		if err != nil {
			return err
		}

		os.Stdin = file
	}

	return nil
}
