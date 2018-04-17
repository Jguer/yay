package main

import "testing"

func TestGetVersionDiff(t *testing.T) {
	useColor = true

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
		{"1-1", "1-1"},
		{red("1-1"), green("2-1")},
		{red("2-1"), green("1-1")},
		{"1-" + red("1"), "1-" + green("2")},
		{"1-" + red("2"), "1-" + green("1")},
		{"1.2." + red("3-1"), "1.2." + green("4-1")},
		{"1.8rc1+" + red("6+g0f377f94-1"), "1.8rc1+" + green("1+g7e949283-1")},
		{"1.8" + red("rc1+6+g0f377f94-1"), "1.8" + green("rc2+1+g7e949283-1")},
		{"1." + red("8rc2"), "1." + green("9rc1")},
		{"2.99.917+" + red("812+g75795523-1"), "2.99.917+" + green("823+gd9bf46e4-1")},
		{"1.2." + red("9-1"), "1.2." + green("10-1")},
		{"1.2." + red("10-1"), "1.2." + green("9-1")},
		{"1.2" + red("-1"), "1.2" + green(".1-1")},
		{"1.2" + red(".1-1"), "1.2" + green("-1")},
		{"0.7" + red("-4"), "0.7" + green("+4+gd8d8c67-1")},
		{"1.0.2_r0-" + red("1"), "1.0.2_r0-" + green("2")},
		{"1.0.2_" + red("r0-1"), "1.0.2_" + green("r1-1")},
		{"1.0." + red("2_r0-1"), "1.0." + green("3_r0-1")},
	}

	for i, pair := range in {
		o, n := getVersionDiff(pair.Old, pair.New)

		if o != out[i].Old || n != out[i].New {
			t.Errorf("Test %d failed for update: (%s => %s) expected (%s => %s) got (%s => %s)", i+1, in[i].Old, in[i].New, out[i].Old, out[i].New, o, n)
		}
	}
}
