package pacman

import "testing"
import "github.com/jguer/yay/util"
import "os"

func benchmarkPrintSearch(search string, b *testing.B) {
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	for n := 0; n < b.N; n++ {
		res, _, _ := Search(search)
		res.PrintSearch()
	}
	os.Stdout = old
}

func BenchmarkPrintSearchSimpleTopDown(b *testing.B) {
	util.SortMode = util.TopDown
	benchmarkPrintSearch("chromium", b)
}
func BenchmarkPrintSearchComplexTopDown(b *testing.B) {
	util.SortMode = util.TopDown
	benchmarkPrintSearch("linux", b)
}

func BenchmarkPrintSearchSimpleBottomUp(b *testing.B) {
	util.SortMode = util.BottomUp
	benchmarkPrintSearch("chromium", b)
}
func BenchmarkPrintSearchComplexBottomUp(b *testing.B) {
	util.SortMode = util.BottomUp
	benchmarkPrintSearch("linux", b)
}

func benchmarkSearch(search string, b *testing.B) {
	for n := 0; n < b.N; n++ {
		Search(search)
	}
}
func BenchmarkSearchSimpleTopDown(b *testing.B) {
	util.SortMode = util.TopDown
	benchmarkSearch("chromium", b)
}

func BenchmarkSearchSimpleBottomUp(b *testing.B) {
	util.SortMode = util.BottomUp
	benchmarkSearch("chromium", b)
}

func BenchmarkSearchComplexTopDown(b *testing.B) {
	util.SortMode = util.TopDown
	benchmarkSearch("linux", b)
}
func BenchmarkSearchComplexBottomUp(b *testing.B) {
	util.SortMode = util.BottomUp
	benchmarkSearch("linux", b)
}
