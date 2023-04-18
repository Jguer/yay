package query

import (
	"testing"

	"github.com/Jguer/yay/v12/pkg/text"
)

func TestVersionDiff(t *testing.T) {
	testCases := []struct {
		name     string
		a        string
		b        string
		wantDiff string
	}{
		{
			name:     "1.0.0-1 -> 1.0.0-2",
			a:        "1.0.0-1",
			b:        "1.0.0-2",
			wantDiff: "1.0.0-" + text.Red("1") + " " + "1.0.0-" + text.Green("2"),
		},
		{
			name:     "1.0.0-1 -> 1.0.1-1",
			a:        "1.0.0-1",
			b:        "1.0.1-1",
			wantDiff: "1.0." + text.Red("0-1") + " " + "1.0." + text.Green("1-1"),
		},
		{
			name:     "3.0.0~alpha7-3 -> 3.0.0~alpha7-4",
			a:        "3.0.0~alpha7-3",
			b:        "3.0.0~alpha7-4",
			wantDiff: "3.0.0~alpha7-" + text.Red("3") + " " + "3.0.0~alpha7-" + text.Green("4"),
		},
		{
			name:     "3.0.0~beta7-3 -> 3.0.0~beta8-3",
			a:        "3.0.0~beta7-3",
			b:        "3.0.0~beta8-3",
			wantDiff: "3.0.0~" + text.Red("beta7-3") + " " + "3.0.0~" + text.Green("beta8-3"),
		},
		{
			name:     "23.04.r131.b1bfe05-1 -> 23.04.r131.b1bfe07-1",
			a:        "23.04.r131.b1bfe05-1",
			b:        "23.04.r131.b1bfe07-1",
			wantDiff: "23.04.r131." + text.Red("b1bfe05-1") + " " + "23.04.r131." + text.Green("b1bfe07-1"),
		},
		{
			name:     "1.0.arch0-1 -> 1.0.arch1-2",
			a:        "1.0.arch0-1",
			b:        "1.0.arch1-2",
			wantDiff: "1.0." + text.Red("arch0-1") + " " + "1.0." + text.Green("arch1-2"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			originalUseColor := text.UseColor
			text.UseColor = true
			left, right := GetVersionDiff(tc.a, tc.b)
			gotDiff := left + " " + right
			if gotDiff != tc.wantDiff {
				t.Errorf("VersionDiff(%s, %s) = %s, want %s", tc.a, tc.b, gotDiff, tc.wantDiff)
			}
			text.UseColor = originalUseColor
		})
	}
}
