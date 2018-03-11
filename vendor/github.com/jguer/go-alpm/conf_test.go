// conf_test.go - Tests for conf.go.
//
// Copyright (c) 2013 The go-alpm Authors
//
// MIT Licensed. See LICENSE for details.

package alpm

import (
	"os"
	"reflect"
	"testing"
)

var pacmanConfRef = PacmanConfig{
	RootDir:      "/",
	DBPath:       "/var/lib/pacman",
	CacheDir:     []string{"/var/cache/pacman/pkg", "/other/cachedir"},
	LogFile:      "/var/log/pacman.log",
	GPGDir:       "/etc/pacman.d/gnupg/",
	HoldPkg:      []string{"pacman", "glibc"},
	XferCommand:  "/usr/bin/wget --passive-ftp -c -O %o %u",
	Architecture: "auto",
	CleanMethod:  "KeepInstalled",
	UseDelta:     0.7,
	IgnorePkg:    []string{"hello", "world"},
	IgnoreGroup:  []string{"kde"},
	NoUpgrade:    nil,
	NoExtract:    nil,

	Options: ConfColor | ConfCheckSpace | ConfVerbosePkgLists,

	Repos: []RepoConfig{
		{Name: "core", Servers: []string{"ftp://ftp.example.com/foobar/$repo/os/$arch/"}},
		{Name: "custom", Servers: []string{"file:///home/custompkgs"}},
	},
}

func detailedDeepEqual(t *testing.T, x, y interface{}) {
	v := reflect.ValueOf(x)
	w := reflect.ValueOf(y)
	if v.Type() != w.Type() {
		t.Errorf("differing types %T vs. %T", x, y)
		return
	}
	for i := 0; i < v.NumField(); i++ {
		v_fld := v.Field(i).Interface()
		w_fld := w.Field(i).Interface()
		if !reflect.DeepEqual(v_fld, w_fld) {
			t.Errorf("field %s differs: got %#v, expected %#v",
				v.Type().Field(i).Name, v_fld, w_fld)
		}
	}
}

func TestParseConfigGood(t *testing.T) {
	f, err := os.Open("testing/conf/good_pacman.conf")
	if err != nil {
		t.Error(err)
	}
	conf, err := ParseConfig(f)
	if err != nil {
		t.Error(err)
	}
	detailedDeepEqual(t, conf, pacmanConfRef)
}
