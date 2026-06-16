package lua

import (
	"testing"
)

// makeSearchFilterEngine returns an Engine with one SearchFilter hook that
// returns all results unchanged (a passthrough). Callers must defer e.Close().
func makeSearchFilterEngine(b *testing.B) *Engine {
	b.Helper()

	e := New()
	if err := e.L.DoString(`
		yay.create_autocmd("SearchFilter", {
			callback = function(event)
				local out = {}
				for _, r in ipairs(event.data.results) do
					out[#out + 1] = { source = r.source, name = r.name }
				end
				return out
			end,
		})
	`); err != nil {
		b.Fatalf("lua setup: %v", err)
	}

	return e
}

// makeAURPreInstallEngine returns an Engine with one AURPreInstall hook that
// does nothing. Callers must defer e.Close().
func makeAURPreInstallEngine(b *testing.B) *Engine {
	b.Helper()

	e := New()
	if err := e.L.DoString(`
		yay.create_autocmd("AURPreInstall", {
			callback = function(_event) end,
		})
	`); err != nil {
		b.Fatalf("lua setup: %v", err)
	}

	return e
}

// makeUpgradeSelectEngine returns an Engine with one UpgradeSelect hook that
// returns nil (no exclusions). Callers must defer e.Close().
func makeUpgradeSelectEngine(b *testing.B) *Engine {
	b.Helper()

	e := New()
	if err := e.L.DoString(`
		yay.create_autocmd("UpgradeSelect", {
			callback = function(_event) return nil end,
		})
	`); err != nil {
		b.Fatalf("lua setup: %v", err)
	}

	return e
}

// makePostInstallEngine returns an Engine with one PostInstall hook that does
// nothing. Callers must defer e.Close().
func makePostInstallEngine(b *testing.B) *Engine {
	b.Helper()

	e := New()
	if err := e.L.DoString(`
		yay.create_autocmd("PostInstall", {
			callback = function(_event) end,
		})
	`); err != nil {
		b.Fatalf("lua setup: %v", err)
	}

	return e
}

func makeSearchFilterEvent(n int) *SearchFilterEvent {
	results := make([]SearchResultPackage, n)
	for i := range results {
		results[i] = SearchResultPackage{
			Source:         "aur",
			Name:           "pkg-" + string(rune('a'+i%26)),
			Description:    "benchmark package",
			Votes:          100 + i,
			Popularity:     float64(i) * 0.1,
			FirstSubmitted: 1_600_000_000 + i*1000,
			LastModified:   1_700_000_000 + i*1000,
		}
	}

	return &SearchFilterEvent{Results: results}
}

func makeAURPreInstallEvent() *AURPreInstallEvent {
	return &AURPreInstallEvent{
		Base:    "my-pkg",
		Version: "1.0-1",
		Packages: []AURPreInstallPackage{
			{Name: "my-pkg", Version: "1.0-1", Reason: "explicit"},
		},
		SRCINFO: AURPreInstallSRCINFO{
			Pkgbase: "my-pkg",
			Pkgver:  "1.0",
			Pkgrel:  "1",
		},
	}
}

func makeUpgradeSelectEvent(n int) *UpgradeSelectEvent {
	pkgs := make([]UpgradeSelectPackage, n)
	for i := range pkgs {
		pkgs[i] = UpgradeSelectPackage{
			ID:            i + 1,
			Name:          "pkg-" + string(rune('a'+i%26)),
			Repository:    "aur",
			LocalVersion:  "1.0",
			RemoteVersion: "2.0",
			Reason:        "explicit",
			LastModified:  1_700_000_000 + int64(i)*1000,
		}
	}

	return &UpgradeSelectEvent{Upgrades: pkgs}
}

func makePostInstallEvent(n int) *PostInstallEvent {
	pkgs := make([]PostInstallPackage, n)
	for i := range pkgs {
		pkgs[i] = PostInstallPackage{
			Name:      "pkg-" + string(rune('a'+i%26)),
			Version:   "2.0-1",
			Source:    "aur",
			Reason:    "explicit",
			Installed: true,
			Upgrade:   true,
		}
	}

	return &PostInstallEvent{Packages: pkgs}
}

// BenchmarkRunSearchFilter_10 measures Lua SearchFilter dispatch with 10 packages.
func BenchmarkRunSearchFilter_10(b *testing.B) {
	e := makeSearchFilterEngine(b)
	defer e.Close()

	event := makeSearchFilterEvent(10)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if _, err := e.RunSearchFilter(event); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRunSearchFilter_100 measures Lua SearchFilter dispatch with 100 packages.
func BenchmarkRunSearchFilter_100(b *testing.B) {
	e := makeSearchFilterEngine(b)
	defer e.Close()

	event := makeSearchFilterEvent(100)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if _, err := e.RunSearchFilter(event); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRunAURPreInstall measures the overhead of dispatching an
// AURPreInstall Lua event for a single package.
func BenchmarkRunAURPreInstall(b *testing.B) {
	e := makeAURPreInstallEngine(b)
	defer e.Close()

	event := makeAURPreInstallEvent()

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if err := e.RunAURPreInstall(event); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRunUpgradeSelect_10 measures Lua UpgradeSelect dispatch with 10
// pending upgrades.
func BenchmarkRunUpgradeSelect_10(b *testing.B) {
	e := makeUpgradeSelectEngine(b)
	defer e.Close()

	event := makeUpgradeSelectEvent(10)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if _, err := e.RunUpgradeSelect(event); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRunPostInstall_10 measures Lua PostInstall dispatch with 10
// packages.
func BenchmarkRunPostInstall_10(b *testing.B) {
	e := makePostInstallEngine(b)
	defer e.Close()

	event := makePostInstallEvent(10)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if err := e.RunPostInstall(event); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkHasAutocmd_miss measures the cost of HasAutocmd when no hook is
// registered — the common no-op fast path.
func BenchmarkHasAutocmd_miss(b *testing.B) {
	e := New()
	defer e.Close()

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		e.HasAutocmd(EventSearchFilter)
	}
}
