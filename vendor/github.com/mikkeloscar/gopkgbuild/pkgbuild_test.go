package pkgbuild

import "testing"

// Test version parsing
func TestVersionParsing(t *testing.T) {
	versions := map[string]bool{
		"1.0beta":   true,
		"1.0.0.0.2": true,
		"a.3_4":     true,
		"A.2":       true,
		"_1.2":      false,
		".2":        false,
		"a.2Ø":      false,
		"1.?":       false,
		"1.-":       false,
	}

	for version, valid := range versions {
		_, err := parseVersion(version)
		if err != nil && valid {
			t.Errorf("Version string '%s' should parse", version)
		}

		if err == nil && !valid {
			t.Errorf("Version string '%s' should not parse", version)
		}
	}
}

// Test complete-version parsing
func TestCompleteVersionParsing(t *testing.T) {
	versions := map[string]*CompleteVersion{
		"1:1.0beta": &CompleteVersion{Version("1.0beta"), 1, 0},
		"1.0":       &CompleteVersion{Version("1.0"), 0, 0},
		"2.3-2":     &CompleteVersion{Version("2.3"), 0, 2},
		"1::":       nil,
		"4.3--1":    nil,
		"4.1-a":     nil,
		"f:2.3":     nil,
		"1.?":       nil,
	}

	for version, expected := range versions {
		ver, err := NewCompleteVersion(version)
		if err != nil && expected != nil {
			t.Errorf("CompleteVersion string '%s' should not parse", version)
		}

		if err == nil && expected != nil {
			if ver.Version != expected.Version || ver.Epoch != expected.Epoch || ver.Pkgrel != expected.Pkgrel {
				t.Errorf("CompleteVersion string '%s' should parse", version)
			}
		}
	}
}

// Test Newer method
func TestNewer(t *testing.T) {
	a := &PKGBUILD{
		Epoch:  0,
		Pkgver: Version("1.0"),
		Pkgrel: 1,
	}
	b := &PKGBUILD{
		Epoch:  0,
		Pkgver: Version("2.0"),
		Pkgrel: 1,
	}
	c := &PKGBUILD{
		Epoch:  1,
		Pkgver: Version("1.0"),
		Pkgrel: 1,
	}
	d := &PKGBUILD{
		Epoch:  0,
		Pkgver: Version("1.0"),
		Pkgrel: 2,
	}

	if a.Newer(b) {
		t.Errorf("a (%s) should not be newer than b (%s)", a.Version(), b.Version())
	}

	if b.Newer(c) {
		t.Errorf("b (%s) should not be newer than c (%s)", b.Version(), c.Version())
	}

	if a.Newer(d) {
		t.Errorf("a (%s) should not be newer than d (%s)", a.Version(), d.Version())
	}

	if a.Newer(a) {
		t.Errorf("a (%s) should not be newer than itself", a.Version())
	}
}

// Test Older method
func TestOlder(t *testing.T) {
	a := &PKGBUILD{
		Epoch:  0,
		Pkgver: Version("1.0"),
		Pkgrel: 1,
	}
	b := &PKGBUILD{
		Epoch:  0,
		Pkgver: Version("2.0"),
		Pkgrel: 1,
	}
	c := &PKGBUILD{
		Epoch:  1,
		Pkgver: Version("1.0"),
		Pkgrel: 1,
	}
	d := &PKGBUILD{
		Epoch:  0,
		Pkgver: Version("1.0"),
		Pkgrel: 2,
	}

	if !a.Older(b) {
		t.Errorf("a (%s) should be older than b (%s)", a.Version(), b.Version())
	}

	if !b.Older(c) {
		t.Errorf("b (%s) should be older than c (%s)", b.Version(), c.Version())
	}

	if !a.Older(d) {
		t.Errorf("a (%s) should be older than d (%s)", a.Version(), d.Version())
	}

	if d.Older(d) {
		t.Errorf("d (%s) should not be older than itself", d.Version())
	}
}

// Test Version method
func TestVersionMethod(t *testing.T) {
	a := &PKGBUILD{
		Epoch:  0,
		Pkgver: Version("1.0"),
		Pkgrel: 1,
	}

	version := "1.0-1"

	if a.Version() != version {
		t.Errorf("a (%s) should be %s", a.Version(), version)
	}

	b := &PKGBUILD{
		Epoch:  4,
		Pkgver: Version("1.0"),
		Pkgrel: 1,
	}

	version = "4:1.0-1"

	if b.Version() != version {
		t.Errorf("a (%s) should be %s", b.Version(), version)
	}
}

// Test random SRCINFO files based on pkgbuilds from Arch core
func TestRandomCoreSRCINFOs(t *testing.T) {
	srcinfos := []string{
		"sudo",
		"pacman",
		"openssh",
		"grub",
		"glibc",
		"systemd",
		"linux",
	}

	for _, srcinfo := range srcinfos {
		path := "./test_pkgbuilds/SRCINFO_" + srcinfo
		pkg, err := ParseSRCINFO(path)
		if err != nil {
			t.Errorf("PKGBUILD for %s did not parse: %s", srcinfo, err.Error())
		}

		if pkg.Pkgbase != srcinfo {
			t.Errorf("pkgbase for %s should be %s", srcinfo, pkg.Pkgbase)
		}
	}
}

func TestParseDependency(t *testing.T) {
	deps := make([]*Dependency, 0)
	_, err := parseDependency("linux-mainline-headers<4.6rc1", deps)
	if err != nil {
		t.Errorf("could not parse dependency %s: %s", "bla", err.Error())
	}

	_, err = parseDependency("linux-mainline-headers<=4.6rc1", deps)
	if err != nil {
		t.Errorf("could not parse dependency %s: %s", "bla", err.Error())
	}

	_, err = parseDependency("linux-mainline-headers>=4.6rc1", deps)
	if err != nil {
		t.Errorf("could not parse dependency %s: %s", "bla", err.Error())
	}

	_, err = parseDependency("linux-mainline-headers==4.6rc1", deps)
	if err != nil {
		t.Errorf("could not parse dependency %s: %s", "bla", err.Error())
	}
}
