package pacman

import "testing"
import "github.com/jguer/yay/util"

func benchmarkSearch(search string, b *testing.B) {
	for n := 0; n < b.N; n++ {
		Search(search)
	}
}

func BenchmarkSearchSimpleTopDown(b *testing.B) {
	util.SortMode = TopDown
	benchmarkSearch("chromium", b)
}
func BenchmarkSearchComplexTopDown(b *testing.B) { util.SortMode = TopDown; benchmarkSearch("linux", b) }
func BenchmarkSearchSimpleDownTop(b *testing.B) {
	util.SortMode = DownTop
	benchmarkSearch("chromium", b)
}
func BenchmarkSearchComplexDownTop(b *testing.B) { util.SortMode = DownTop; benchmarkSearch("linux", b) }
