package aur

import (
	"os"
	"reflect"
	"testing"

	"github.com/demizer/go-alpm"
)

func TestSearch(t *testing.T) {
	eN := "yay"
	eD := "Yet another pacman wrapper with AUR support"
	result, _, err := Search("yay", true)
	if err != nil {
		t.Fatalf("Expected err to be nil but it was %s", err)
	}

	// t.Logf("Got struct: %+v", result)
	found := false
	for _, v := range result {
		if v.Name == eN && v.Description == eD {
			found = true
		}
	}

	if !found {
		t.Fatalf("Expected to find yay, found %+v", result)
	}
}

func benchmarkSearch(search string, sort bool, b *testing.B) {
	for n := 0; n < b.N; n++ {
		Search(search, sort)
	}
}

func BenchmarkSearchSimpleNoSort(b *testing.B)  { benchmarkSearch("yay", false, b) }
func BenchmarkSearchComplexNoSort(b *testing.B) { benchmarkSearch("linux", false, b) }
func BenchmarkSearchSimpleSorted(b *testing.B)  { benchmarkSearch("yay", true, b) }
func BenchmarkSearchComplexSorted(b *testing.B) { benchmarkSearch("linux", true, b) }

func TestInfo(t *testing.T) {
	eN := "yay"
	eD := "Yet another pacman wrapper with AUR support"
	eM := []string{"go", "git"}
	result, _, err := Info("yay")
	if err != nil {
		t.Fatalf("Expected err to be nil but it was %s", err)
	}

	// t.Logf("Got struct: %+v", result)
	found := false
	for _, v := range result {
		if v.Name == eN && v.Description == eD && reflect.DeepEqual(v.MakeDepends, eM) {
			found = true
		}
	}

	if !found {
		t.Fatalf("Expected to find yay, found %+v", result)
	}
}

func TestUpdate(t *testing.T) {
	var conf alpm.PacmanConfig
	file, err := os.Open("/etc/pacman.conf")
	if err != nil {
		return
	}
	conf, err = alpm.ParseConfig(file)
	if err != nil {
		return
	}

	err = UpdatePackages("/tmp/yaytmp", &conf, []string{})
	if err != nil {
		t.Fatalf("Expected err to be nil but it was %s", err)
	}
}

func TestUpgrade(t *testing.T) {
	var conf alpm.PacmanConfig
	file, err := os.Open("/etc/pacman.conf")
	if err != nil {
		return
	}
	conf, err = alpm.ParseConfig(file)
	if err != nil {
		return
	}

	err = Upgrade("/tmp/yaytmp", &conf, []string{})
	if err != nil {
		t.Fatalf("Expected err to be nil but it was %s", err)
	}
}

func BenchmarkUpdate(b *testing.B) {
	var conf alpm.PacmanConfig
	file, err := os.Open("/etc/pacman.conf")
	if err != nil {
		return
	}
	conf, err = alpm.ParseConfig(file)
	if err != nil {
		return
	}

	for n := 0; n < b.N; n++ {
		UpdatePackages("/tmp/yaytmp", &conf, []string{})
	}
}

func BenchmarkUpgrade(b *testing.B) {
	var conf alpm.PacmanConfig
	file, err := os.Open("/etc/pacman.conf")
	if err != nil {
		return
	}
	conf, err = alpm.ParseConfig(file)
	if err != nil {
		return
	}

	for n := 0; n < b.N; n++ {
		Upgrade("/tmp/yaytmp", &conf, []string{})
	}
}
