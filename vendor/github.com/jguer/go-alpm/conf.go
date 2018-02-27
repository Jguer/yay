// conf.go - Functions for pacman.conf parsing.
//
// Copyright (c) 2013 The go-alpm Authors
//
// MIT Licensed. See LICENSE for details.

package alpm

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"syscall"
)

type PacmanOption uint

const (
	ConfUseSyslog PacmanOption = 1 << iota
	ConfColor
	ConfTotalDownload
	ConfCheckSpace
	ConfVerbosePkgLists
	ConfILoveCandy
)

var optionsMap = map[string]PacmanOption{
	"UseSyslog":       ConfUseSyslog,
	"Color":           ConfColor,
	"TotalDownload":   ConfTotalDownload,
	"CheckSpace":      ConfCheckSpace,
	"VerbosePkgLists": ConfVerbosePkgLists,
	"ILoveCandy":      ConfILoveCandy,
}

// PacmanConfig is a type for holding pacman options parsed from pacman
// configuration data passed to ParseConfig.
type PacmanConfig struct {
	RootDir            string
	DBPath             string
	CacheDir           []string
	HookDir            []string
	GPGDir             string
	LogFile            string
	HoldPkg            []string
	IgnorePkg          []string
	IgnoreGroup        []string
	Include            []string
	Architecture       string
	XferCommand        string
	NoUpgrade          []string
	NoExtract          []string
	CleanMethod        string
	SigLevel           SigLevel
	LocalFileSigLevel  SigLevel
	RemoteFileSigLevel SigLevel
	UseDelta           float64
	Options            PacmanOption
	Repos              []RepoConfig
}

// RepoConfig is a type that stores the signature level of a repository
// specified in the pacman config file.
type RepoConfig struct {
	Name     string
	SigLevel SigLevel
	Servers  []string
}

// Constants for pacman configuration parsing
const (
	tokenSection = iota
	tokenKey
	tokenComment
)

type iniToken struct {
	Type   uint
	Name   string
	Values []string
}

type confReader struct {
	*bufio.Reader
	Lineno uint
}

// newConfReader reads from the io.Reader if it is buffered and returns a
// confReader containing the number of bytes read and 0 for the first line. If
// r is not a buffered reader, a new buffered reader is created using r as its
// input and returned.
func newConfReader(r io.Reader) confReader {
	if buf, ok := r.(*bufio.Reader); ok {
		return confReader{buf, 0}
	}
	buf := bufio.NewReader(r)
	return confReader{buf, 0}
}

func (rdr *confReader) ParseLine() (tok iniToken, err error) {
	line, overflow, err := rdr.ReadLine()
	switch {
	case err != nil:
		return
	case overflow:
		err = fmt.Errorf("line %d too long", rdr.Lineno)
		return
	}
	rdr.Lineno++

	line = bytes.TrimSpace(line)

	comment := bytes.IndexByte(line, '#')
	if comment >= 0 {
		line = line[:comment]
	}

	if len(line) == 0 {
		tok.Type = tokenComment
		return
	}

	switch line[0] {
	case '[':
		closing := bytes.IndexByte(line, ']')
		if closing < 0 {
			err = fmt.Errorf("missing ']' is section name at line %d", rdr.Lineno)
			return
		}
		tok.Name = string(line[1:closing])
		if closing+1 < len(line) {
			err = fmt.Errorf("trailing characters %q after section name %s",
				line[closing+1:], tok.Name)
			return
		}
		return
	default:
		tok.Type = tokenKey
		if idx := bytes.IndexByte(line, '='); idx >= 0 {
			optname := bytes.TrimSpace(line[:idx])
			values := bytes.Split(line[idx+1:], []byte{' '})
			tok.Name = string(optname)
			tok.Values = make([]string, 0, len(values))
			for _, word := range values {
				word = bytes.TrimSpace(word)
				if len(word) > 0 {
					tok.Values = append(tok.Values, string(word))
				}
			}
		} else {
			// boolean option
			tok.Name = string(line)
			tok.Values = nil
		}
		return
	}
}

func ParseConfig(r io.Reader) (conf PacmanConfig, err error) {
	rdr := newConfReader(r)
	rdrStack := []confReader{rdr}
	conf.SetDefaults()
	confReflect := reflect.ValueOf(&conf).Elem()
	var currentSection string
	var curRepo *RepoConfig
lineloop:
	for {
		line, err := rdr.ParseLine()
		// fmt.Printf("%+v\n", line)
		switch err {
		case io.EOF:
			// pop reader stack.
			l := len(rdrStack)
			if l == 1 {
				break lineloop
			}
			rdr = rdrStack[l-2]
			rdrStack = rdrStack[:l-1]
		default:
			break lineloop
		case nil:
			// Ok.
		}

		switch line.Type {
		case tokenComment:
		case tokenSection:
			currentSection = line.Name
			if currentSection != "options" {
				conf.Repos = append(conf.Repos, RepoConfig{})
				curRepo = &conf.Repos[len(conf.Repos)-1]
				curRepo.Name = line.Name
			}
		case tokenKey:
			switch line.Name {
			case "SigLevel":
				// TODO: implement SigLevel parsing.
				continue lineloop
			case "Server":
				curRepo.Servers = append(curRepo.Servers, line.Values...)
				continue lineloop
			case "Include":
				f, err := os.Open(line.Values[0])
				if err != nil {
					err = fmt.Errorf("error while processing Include directive at line %d: %s",
						rdr.Lineno, err)
					break lineloop
				}
				rdr = newConfReader(f)
				rdrStack = append(rdrStack, rdr)
				continue lineloop
			case "UseDelta":
				if len(line.Values) > 0 {
					deltaRatio, err := strconv.ParseFloat(line.Values[0], 64)

					if err != nil {
						break lineloop
					}

					conf.UseDelta = deltaRatio
				}
				continue lineloop
			}

			if currentSection != "options" {
				err = fmt.Errorf("option %s outside of [options] section, at line %d",
					line.Name, rdr.Lineno)
				break lineloop
			}
			// main options.
			if opt, ok := optionsMap[line.Name]; ok {
				// boolean option.
				conf.Options |= opt
			} else {
				// key-value option.
				fld := confReflect.FieldByName(line.Name)
				if !fld.IsValid() || !fld.CanAddr() {
					_ = fmt.Errorf("unknown option at line %d: %s", rdr.Lineno, line.Name)
					continue
				}

				switch fieldP := fld.Addr().Interface().(type) {
				case *string:
					// single valued option.
					*fieldP = strings.Join(line.Values, " ")
				case *[]string:
					//many valued option.
					*fieldP = append(*fieldP, line.Values...)
				}
			}
		}
	}

	if len(conf.CacheDir) == 0 {
		conf.CacheDir = []string{"/var/cache/pacman/pkg/"} //should only be set if the config does not specify this
	}

	return conf, err
}

func (conf *PacmanConfig) SetDefaults() {
	conf.RootDir = "/"
	conf.DBPath = "/var/lib/pacman"
	conf.DBPath = "/var/lib/pacman/"
	conf.HookDir = []string{"/etc/pacman.d/hooks/"} //should be added to whatever the config states
	conf.GPGDir = "/etc/pacman.d/gnupg/"
	conf.LogFile = "/var/log/pacman.log"
	conf.UseDelta = 0.7
	conf.CleanMethod = "KeepInstalled"

	conf.SigLevel = SigPackage | SigPackageOptional | SigDatabase | SigDatabaseOptional
	conf.LocalFileSigLevel = SigUseDefault
	conf.RemoteFileSigLevel = SigUseDefault
}

func getArch() (string, error) {
	var uname syscall.Utsname
	err := syscall.Uname(&uname)
	if err != nil {
		return "", err
	}
	var arch [65]byte
	for i, c := range uname.Machine {
		if c == 0 {
			return string(arch[:i]), nil
		}
		arch[i] = byte(c)
	}
	return string(arch[:]), nil
}

func (conf *PacmanConfig) CreateHandle() (*Handle, error) {
	h, err := Init(conf.RootDir, conf.DBPath)
	if err != nil {
		return nil, err
	}
	if conf.Architecture == "auto" {
		conf.Architecture, err = getArch()
		if err != nil {
			return nil, fmt.Errorf("architecture is 'auto' but couldn't uname()")
		}
	}

	for _, repoconf := range conf.Repos {
		// TODO: set SigLevel
		db, err := h.RegisterSyncDb(repoconf.Name, 0)
		if err == nil {
			for i, addr := range repoconf.Servers {
				addr = strings.Replace(addr, "$repo", repoconf.Name, -1)
				addr = strings.Replace(addr, "$arch", conf.Architecture, -1)
				repoconf.Servers[i] = addr
			}
			db.SetServers(repoconf.Servers)
		}
	}

	err = h.SetCacheDirs(conf.CacheDir...)
	if err != nil {
		return nil, err
	}
		
	// add hook directories 1-by-1 to avoid overwriting the system directory
	for _,dir := range conf.HookDir {
		err = h.AddHookDir(dir)
		if err != nil {
			return nil, err
		}
	}

	err = h.SetGPGDir(conf.GPGDir)
	if err != nil {
		return nil, err
	}

	err = h.SetLogFile(conf.LogFile)
	if err != nil {
		return nil, err
	}

	err = h.SetIgnorePkgs(conf.IgnorePkg...)
	if err != nil {
		return nil, err
	}

	err = h.SetIgnoreGroups(conf.IgnoreGroup...)
	if err != nil {
		return nil, err
	}

	err = h.SetArch(conf.Architecture)
	if err != nil {
		return nil, err
	}
	
	h.SetNoUpgrades(conf.NoUpgrade...)
	if err != nil {
		return nil, err
	}

	h.SetNoExtracts(conf.NoExtract...)
	if err != nil {
		return nil, err
	}

	err = h.SetDefaultSigLevel(conf.SigLevel)
	if err != nil {
		return nil, err
	}

	err = h.SetLocalFileSigLevel(conf.LocalFileSigLevel)
	if err != nil {
		return nil, err
	}


	err = h.SetRemoteFileSigLevel(conf.RemoteFileSigLevel)
	if err != nil {
		return nil, err
	}

	err = h.SetDeltaRatio(conf.UseDelta)
	if err != nil {
		return nil, err
	}

	err = h.SetUseSyslog(conf.Options & ConfUseSyslog > 0)
	if err != nil {
		return nil, err
	}

	err = h.SetCheckSpace(conf.Options & ConfCheckSpace > 0)
	if err != nil {
		return nil, err
	}

	return h, nil
}
