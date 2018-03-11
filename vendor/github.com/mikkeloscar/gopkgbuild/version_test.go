package pkgbuild

import "testing"

// Test version comparison
func TestVersionComparison(t *testing.T) {
	alphaNumeric := []Version{
		"1.0.1",
		"1.0.a",
		"1.0",
		"1.0rc",
		"1.0pre",
		"1.0p",
		"1.0beta",
		"1.0b",
		"1.0a",
	}
	numeric := []Version{
		"20141130",
		"012",
		"11",
		"3.0.0",
		"2.011",
		"2.03",
		"2.0",
		"1.2",
		"1.1.1",
		"1.1",
		"1.0.1",
		"1.0.0.0.0.0",
		"1.0",
		"1",
	}
	git := []Version{
		"r1000.b481c3c",
		"r37.e481c3c",
		"r36.f481c3c",
	}

	bigger := func(list []Version) {
		for i, v := range list {
			for _, v2 := range list[i:] {
				if v != v2 && !v.bigger(v2) {
					t.Errorf("%s should be bigger than %s", v, v2)
				}
			}
		}
	}

	smaller := func(list []Version) {
		for i := len(list) - 1; i >= 0; i-- {
			v := list[i]
			for _, v2 := range list[:i] {
				if v != v2 && v.bigger(v2) {
					t.Errorf("%s should be smaller than %s", v, v2)
				}
			}
		}
	}

	bigger(alphaNumeric)
	smaller(alphaNumeric)
	bigger(numeric)
	smaller(numeric)
	bigger(git)
	smaller(git)
}

// Test alphaCompare function
func TestAlphaCompare(t *testing.T) {
	if alphaCompare([]rune("test"), []rune("test")) != 0 {
		t.Error("should be 0")
	}

	if alphaCompare([]rune("test"), []rune("test123")) > 0 {
		t.Error("should be less than 0")
	}

	if alphaCompare([]rune("test123"), []rune("test")) < 0 {
		t.Error("should be greater than 0")
	}
}

// Test CompleteVersion comparisons
func TestCompleteVersionComparison(t *testing.T) {
	a := &CompleteVersion{
		Version: "2",
		Epoch:   1,
		Pkgrel:  Version("2"),
	}

	older := []string{
		"0:3-4",
		"1:2-1",
		"1:2-1.5",
		"1:1-1",
	}

	for _, o := range older {
		if _, err := NewCompleteVersion(o); err != nil {
			t.Errorf("%s fails to parse %v", o, err)
		}
		if a.Older(o) || !a.Newer(o) {
			t.Errorf("%s should be older than %s", o, a.String())
		}
	}

	newer := []string{
		"2:1-1",
		"1:3-1",
		"1:2-3",
		"1:2-2.1",
	}

	for _, n := range newer {
		if _, err := NewCompleteVersion(n); err != nil {
			t.Errorf("%s fails to parse %v", n, err)
		}
		if a.Newer(n) || !a.Older(n) {
			t.Errorf("%s should be newer than %s", n, a.String())
		}
	}
}

func TestCompleteVersionString(t *testing.T) {
	str := "42:3.14-1"
	version, _ := NewCompleteVersion(str)
	if version.String() != str {
		t.Errorf("%v should equal %s", version, str)
	}
}

// Benchmark rpmvercmp
func BenchmarkVersionCompare(b *testing.B) {
	for i := 0; i < b.N; i++ {
		rpmvercmp("1.0", "1.0.0")
	}
}

func TestUnicode(t *testing.T) {
	str := "13:2.0.0.α.r29.g18fc492-1"
	expected := CompleteVersion{
		Epoch:   13,
		Version: "2.0.0.α.r29.g18fc492",
		Pkgrel:  "1",
	}
	version, err := NewCompleteVersion(str)
	if err != nil {
		t.Error(err)
	} else if *version != expected {
		t.Errorf("%v should be %v", version, expected)
	}
}
