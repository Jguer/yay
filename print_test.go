package main

import (
	"os"
	"testing"
)

func benchmarkPrintSearch(search string, b *testing.B) {
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	for n := 0; n < b.N; n++ {
		res, _, _ := queryRepo(append([]string{}, search))
		res.printSearch()
	}
	os.Stdout = old
}

func BenchmarkPrintSearchSimpleTopDown(b *testing.B) {
	config.SortMode = TopDown
	benchmarkPrintSearch("chromium", b)
}
func BenchmarkPrintSearchComplexTopDown(b *testing.B) {
	config.SortMode = TopDown
	benchmarkPrintSearch("linux", b)
}

func BenchmarkPrintSearchSimpleBottomUp(b *testing.B) {
	config.SortMode = BottomUp
	benchmarkPrintSearch("chromium", b)
}
func BenchmarkPrintSearchComplexBottomUp(b *testing.B) {
	config.SortMode = BottomUp
	benchmarkPrintSearch("linux", b)
}
