package pacman

import "testing"

func benchmarkSearch(search string, b *testing.B) {
	for n := 0; n < b.N; n++ {
		Search(search)
	}
}

func BenchmarkSearchSimpleTopDown(b *testing.B)  { SortMode = TopDown; benchmarkSearch("chromium", b) }
func BenchmarkSearchComplexTopDown(b *testing.B) { SortMode = TopDown; benchmarkSearch("linux", b) }
func BenchmarkSearchSimpleDownTop(b *testing.B)  { SortMode = DownTop; benchmarkSearch("chromium", b) }
func BenchmarkSearchComplexDownTop(b *testing.B) { SortMode = DownTop; benchmarkSearch("linux", b) }
