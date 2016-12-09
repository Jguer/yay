package aur

import (
	"reflect"
	"testing"
)

func TestSearch(t *testing.T) {
	eN := "yay"
	eD := "Yet another yogurt. Pacman wrapper with AUR support written in go."
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
	eD := "Yet another yogurt. Pacman wrapper with AUR support written in go."
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

func TestUpgrade(t *testing.T) {
	err := Upgrade([]string{})
	if err != nil {
		t.Fatalf("Expected err to be nil but it was %s", err)
	}
}

func BenchmarkUpgrade(b *testing.B) {
	for n := 0; n < b.N; n++ {
		Upgrade([]string{})
	}
}
