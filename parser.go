package main

import (
	"os"
	"fmt"
	"strings"
	"io"
)

type set  map[string]struct{}

type argParser struct {
	op string
	options map[string]string
	doubles set //tracks args passed twice such as -yy and -dd
	targets set
}

func makeArgParser() *argParser {
	return &argParser {
		"",
		make(map[string]string),
		make(set),
		make(set),
	}
}

func (praser *argParser) delArg(option string) {
	delete(praser.options, option)
	delete(praser.doubles, option)
}

func (parser *argParser) needRoot() bool {
	if parser.existsArg("h") || parser.existsArg("help") {
		return false
	}
	
	if parser.existsArg("p") || parser.existsArg("print") {
			return false
	}
	
	switch parser.op {
	case "V", "version":
		return false
	case "D", "database":
		return true
	case "F", "files":
		if parser.existsArg("y") || parser.existsArg("refresh") {
			return true
		}
		return false
	case "Q", "query":
		return false
	case "R", "remove":
		return true
	case "S", "sync":
		if parser.existsArg("s") || parser.existsArg("search") {
			return false
		}
		if parser.existsArg("l") || parser.existsArg("list") {
			return false
		}
		return true
	case "T", "deptest":
		return false
	case "U", "upgrade":
		return true
        
    //yay specific
	case "Y", "yay":
		return false
	case "G", "getpkgbuild":
		return false
	default:
		return false
	}
}

func (praser *argParser) addOP(op string) (err error) {
	if praser.op != "" {
		err = fmt.Errorf("only one operation may be used at a time")
		return
	}
	
	praser.op = op
	return
}

func (praser *argParser) addParam(option string, arg string) (err error) {
	if isOp(option) {
		err = praser.addOP(option)
		return
	}
	
	if praser.existsArg(option) {
		praser.doubles[option] = struct{}{}
	} else {
		praser.options[option] = arg
	}
	
	return
}

func (praser *argParser) addArg(option string) (err error) {
	err = praser.addParam(option, "")
	return
}

func (praser *argParser) existsArg(option string) (ok bool) {
	_, ok = praser.options[option]
	return ok
}

func (praser *argParser) getArg(option string) (arg string, double bool, exists bool) {
	arg, exists = praser.options[option]
	_, double = praser.doubles[option]
	return
}

func (praser *argParser) addTarget(target string) {
	praser.targets[target] = struct{}{}
}

func (praser *argParser) delTarget(target string) {
	delete(praser.targets, target)
}

func (parser *argParser) existsDouble(option string) bool {
	_, ok := parser.doubles[option]
	return ok
}

func (parser *argParser) formatArgs() (args []string) {
	op := formatArg(parser.op)
	args = append(args, op)
	
	for option, arg := range parser.options {
		option = formatArg(option)
		args = append(args, option)
		
		if arg != "" {
			args = append(args, arg)
		}
		
		if parser.existsDouble(option) {
			args = append(args, option)
		}
	}
	
	for target := range parser.targets {
		args = append(args, target)
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
        
    //yay specific
	case "Y", "yay":
		return true
	case "G", "getpkgbuild":
		return true
	default:
		return false
	}
}

func isYayParam(arg string) bool {
	switch arg {
	case "afterclean":
		return true
	case "noafterclean":
		return true
	case "devel":
		return true
	case "nodevel":
		return true
	case "timeupdate":
		return true
	case "notimeupdate":
		return true
	case "topdown":
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
	default:
		return false
	}
}

//parses short hand options such as:
//-Syu -b/some/path -
func (parser *argParser) parseShortOption(arg string, param string) (usedNext bool, err error) {
	if arg == "-" {
		err = parser.addArg("-")
		return
	}
	
	arg = arg[1:]
	
	for k, _char := range arg {
		char := string(_char)
		
		if hasParam(char) {
			if k < len(arg) - 2 {
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

//parses full length options such as:
//--sync --refresh --sysupgrade --dbpath /some/path --
func (parser *argParser) parseLongOption(arg string, param string) (usedNext bool, err error){
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

func (parser *argParser) parseStdin() (err error) {
	for true {
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

func (parser *argParser)parseCommandLine() (err error) {
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
		
		if k + 1 < len(args) {
			nextArg = args[k + 1]
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
	
	return
}