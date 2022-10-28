package upgrade

import (
	"testing"

	"github.com/Jguer/yay/v11/pkg/text"
)

func TestGetVersionDiff(t *testing.T) {
	t.Parallel()
	text.UseColor = true

	type versionPair struct {
		Old string
		New string
	}

	in := []versionPair{
		{"1-1", "1-1"},
		{"1-1", "2-1"},
		{"2-1", "1-1"},
		{"1-1", "1-2"},
		{"1-2", "1-1"},
		{"1.2.3-1", "1.2.4-1"},
		{"1.8rc1+6+g0f377f94-1", "1.8rc1+1+g7e949283-1"},
		{"1.8rc1+6+g0f377f94-1", "1.8rc2+1+g7e949283-1"},
		{"1.8rc2", "1.9rc1"},
		{"2.99.917+812+g75795523-1", "2.99.917+823+gd9bf46e4-1"},
		{"1.2.9-1", "1.2.10-1"},
		{"1.2.10-1", "1.2.9-1"},
		{"1.2-1", "1.2.1-1"},
		{"1.2.1-1", "1.2-1"},
		{"0.7-4", "0.7+4+gd8d8c67-1"},
		{"1.0.2_r0-1", "1.0.2_r0-2"},
		{"1.0.2_r0-1", "1.0.2_r1-1"},
		{"1.0.2_r0-1", "1.0.3_r0-1"},
	}

	out := []versionPair{
		{"1-1" + text.Red(""), "1-1" + text.Green("")},
		{text.Red("1-1"), text.Green("2-1")},
		{text.Red("2-1"), text.Green("1-1")},
		{"1-" + text.Red("1"), "1-" + text.Green("2")},
		{"1-" + text.Red("2"), "1-" + text.Green("1")},
		{"1.2." + text.Red("3-1"), "1.2." + text.Green("4-1")},
		{"1.8rc1+" + text.Red("6+g0f377f94-1"), "1.8rc1+" + text.Green("1+g7e949283-1")},
		{"1.8" + text.Red("rc1+6+g0f377f94-1"), "1.8" + text.Green("rc2+1+g7e949283-1")},
		{"1." + text.Red("8rc2"), "1." + text.Green("9rc1")},
		{"2.99.917+" + text.Red("812+g75795523-1"), "2.99.917+" + text.Green("823+gd9bf46e4-1")},
		{"1.2." + text.Red("9-1"), "1.2." + text.Green("10-1")},
		{"1.2." + text.Red("10-1"), "1.2." + text.Green("9-1")},
		{"1.2" + text.Red("-1"), "1.2" + text.Green(".1-1")},
		{"1.2" + text.Red(".1-1"), "1.2" + text.Green("-1")},
		{"0.7" + text.Red("-4"), "0.7" + text.Green("+4+gd8d8c67-1")},
		{"1.0.2_r0-" + text.Red("1"), "1.0.2_r0-" + text.Green("2")},
		{"1.0.2_" + text.Red("r0-1"), "1.0.2_" + text.Green("r1-1")},
		{"1.0." + text.Red("2_r0-1"), "1.0." + text.Green("3_r0-1")},
	}

	for i, pair := range in {
		o, n := GetVersionDiff(pair.Old, pair.New)

		if o != out[i].Old || n != out[i].New {
			t.Errorf("Test %-2d failed for update: expected (%s => %s) got (%s => %s) %d %d %d %d",
				i+1, out[i].Old, out[i].New, o, n, len(out[i].Old), len(out[i].New), len(o), len(n))
		}
	}
}
