package aur

import (
	"os"
	"reflect"
	"testing"
)

func TestSearch(t *testing.T) {

	eN := "yay"
	result, _, err := Search("yay", true)
	if err != nil {
		t.Fatalf("Expected err to be nil but it was %s", err)
	}

	// t.Logf("Got struct: %+v", result)
	found := false
	for _, v := range result {
		if v.Name == eN {
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
	eM := []string{"go", "git"}
	result, _, err := Info("yay")
	if err != nil {
		t.Fatalf("Expected err to be nil but it was %s", err)
	}

	// t.Logf("Got struct: %+v", result)
	found := false
	for _, v := range result {
		if v.Name == eN && reflect.DeepEqual(v.MakeDepends, eM) {
			found = true
		}
	}

	if !found {
		t.Fatalf("Expected to find yay, found %+v", result)
	}
}

func TestUpgrade(t *testing.T) {
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := Upgrade([]string{})
	if err != nil {
		t.Fatalf("Expected err to be nil but it was %s", err)
	}

	os.Stdout = old
}

func BenchmarkUpgrade(b *testing.B) {
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	for n := 0; n < b.N; n++ {
		Upgrade([]string{})
	}

	os.Stdout = old
}
