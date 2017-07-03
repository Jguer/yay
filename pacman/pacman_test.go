package pacman

import (
	"os"
	"testing"

	"github.com/jguer/yay/config"
)

func benchmarkPrintSearch(search string, b *testing.B) {
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	for n := 0; n < b.N; n++ {
		res, _, _ := Search(append([]string{}, search))
		res.PrintSearch()
	}
	os.Stdout = old
}

func BenchmarkPrintSearchSimpleTopDown(b *testing.B) {
	config.YayConf.SortMode = config.TopDown
	benchmarkPrintSearch("chromium", b)
}
func BenchmarkPrintSearchComplexTopDown(b *testing.B) {
	config.YayConf.SortMode = config.TopDown
	benchmarkPrintSearch("linux", b)
}

func BenchmarkPrintSearchSimpleBottomUp(b *testing.B) {
	config.YayConf.SortMode = config.BottomUp
	benchmarkPrintSearch("chromium", b)
}
func BenchmarkPrintSearchComplexBottomUp(b *testing.B) {
	config.YayConf.SortMode = config.BottomUp
	benchmarkPrintSearch("linux", b)
}

func benchmarkSearch(search string, b *testing.B) {
	for n := 0; n < b.N; n++ {
		Search(append([]string{}, search))
	}
}
func BenchmarkSearchSimpleTopDown(b *testing.B) {
	config.YayConf.SortMode = config.TopDown
	benchmarkSearch("chromium", b)
}

func BenchmarkSearchSimpleBottomUp(b *testing.B) {
	config.YayConf.SortMode = config.BottomUp
	benchmarkSearch("chromium", b)
}

func BenchmarkSearchComplexTopDown(b *testing.B) {
	config.YayConf.SortMode = config.TopDown
	benchmarkSearch("linux", b)
}
func BenchmarkSearchComplexBottomUp(b *testing.B) {
	config.YayConf.SortMode = config.BottomUp
	benchmarkSearch("linux", b)
}
