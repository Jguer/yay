package lua

import (
	"fmt"

	glua "github.com/yuin/gopher-lua"
)

const EventAURPreInstall = "AURPreInstall"

type Autocmd struct {
	Event    string
	Desc     string
	callback *glua.LFunction
}

type AURPreInstallEvent struct {
	Base         string
	Dir          string
	PKGBUILDPath string
	SRCINFOPath  string
	PKGBUILD     string
	Version      string
	LastModified int64
	Installed    bool
	Packages     []AURPreInstallPackage
	SRCINFO      AURPreInstallSRCINFO
}

type AURPreInstallPackage struct {
	Name         string
	Version      string
	LocalVersion string
	Reason       string
	Upgrade      bool
	Devel        bool
}

type AURPreInstallSRCINFO struct {
	Pkgbase      string
	Pkgver       string
	Pkgrel       string
	Epoch        string
	Version      string
	Pkgdesc      string
	URL          string
	Arch         []string
	License      []string
	Depends      []string
	MakeDepends  []string
	CheckDepends []string
	OptDepends   []string
	Provides     []string
	Conflicts    []string
	Replaces     []string
}

func (e *Engine) createAutocmd(state *glua.LState) int {
	event := state.CheckString(1)
	if event != EventAURPreInstall {
		state.ArgError(1, fmt.Sprintf("unsupported event %q", event))
		return 0
	}

	opts := state.CheckTable(2)
	callback := state.GetField(opts, "callback")
	fn, ok := callback.(*glua.LFunction)
	if !ok {
		state.ArgError(2, "callback must be a function")
		return 0
	}

	desc := ""
	if val := state.GetField(opts, "desc"); val != glua.LNil {
		str, ok := val.(glua.LString)
		if !ok {
			state.ArgError(2, "desc must be a string")
			return 0
		}
		desc = string(str)
	}

	e.autocmds[event] = append(e.autocmds[event], Autocmd{
		Event:    event,
		Desc:     desc,
		callback: fn,
	})

	return 0
}

func (e *Engine) HasAutocmd(event string) bool {
	return e != nil && len(e.autocmds[event]) > 0
}

func (e *Engine) RunAURPreInstall(event *AURPreInstallEvent) error {
	if !e.HasAutocmd(EventAURPreInstall) {
		return nil
	}

	for _, autocmd := range e.autocmds[EventAURPreInstall] {
		if err := e.L.CallByParam(glua.P{
			Fn:      autocmd.callback,
			NRet:    0,
			Protect: true,
		}, e.aurPreInstallTable(event)); err != nil {
			wrapped := err
			if abortErr, ok := luaAbortError(err); ok {
				wrapped = abortErr
			}

			return fmt.Errorf("%s %s: %w", EventAURPreInstall, event.Base, wrapped)
		}
	}

	return nil
}

func (e *Engine) aurPreInstallTable(event *AURPreInstallEvent) *glua.LTable {
	state := e.L
	eventTable := state.NewTable()
	data := state.NewTable()

	eventTable.RawSetString("event", glua.LString(EventAURPreInstall))
	eventTable.RawSetString("match", glua.LString(event.Base))
	eventTable.RawSetString("data", data)

	data.RawSetString("base", glua.LString(event.Base))
	data.RawSetString("dir", glua.LString(event.Dir))
	data.RawSetString("pkgbuild_path", glua.LString(event.PKGBUILDPath))
	data.RawSetString("srcinfo_path", glua.LString(event.SRCINFOPath))
	data.RawSetString("pkgbuild", glua.LString(event.PKGBUILD))
	data.RawSetString("version", glua.LString(event.Version))
	data.RawSetString("last_modified", glua.LNumber(event.LastModified))
	data.RawSetString("installed", glua.LBool(event.Installed))
	data.RawSetString("packages", e.packagesTable(event.Packages))
	data.RawSetString("srcinfo", e.srcinfoTable(&event.SRCINFO))

	return eventTable
}

func (e *Engine) packagesTable(packages []AURPreInstallPackage) *glua.LTable {
	state := e.L
	tbl := state.NewTable()

	for _, pkg := range packages {
		pkgTbl := state.NewTable()
		pkgTbl.RawSetString("name", glua.LString(pkg.Name))
		pkgTbl.RawSetString("version", glua.LString(pkg.Version))
		pkgTbl.RawSetString("local_version", glua.LString(pkg.LocalVersion))
		pkgTbl.RawSetString("reason", glua.LString(pkg.Reason))
		pkgTbl.RawSetString("upgrade", glua.LBool(pkg.Upgrade))
		pkgTbl.RawSetString("devel", glua.LBool(pkg.Devel))
		tbl.Append(pkgTbl)
	}

	return tbl
}

func (e *Engine) srcinfoTable(srcinfo *AURPreInstallSRCINFO) *glua.LTable {
	state := e.L
	tbl := state.NewTable()

	tbl.RawSetString("pkgbase", glua.LString(srcinfo.Pkgbase))
	tbl.RawSetString("pkgver", glua.LString(srcinfo.Pkgver))
	tbl.RawSetString("pkgrel", glua.LString(srcinfo.Pkgrel))
	tbl.RawSetString("epoch", glua.LString(srcinfo.Epoch))
	tbl.RawSetString("version", glua.LString(srcinfo.Version))
	tbl.RawSetString("pkgdesc", glua.LString(srcinfo.Pkgdesc))
	tbl.RawSetString("url", glua.LString(srcinfo.URL))
	tbl.RawSetString("arch", e.stringArray(srcinfo.Arch))
	tbl.RawSetString("license", e.stringArray(srcinfo.License))
	tbl.RawSetString("depends", e.stringArray(srcinfo.Depends))
	tbl.RawSetString("makedepends", e.stringArray(srcinfo.MakeDepends))
	tbl.RawSetString("checkdepends", e.stringArray(srcinfo.CheckDepends))
	tbl.RawSetString("optdepends", e.stringArray(srcinfo.OptDepends))
	tbl.RawSetString("provides", e.stringArray(srcinfo.Provides))
	tbl.RawSetString("conflicts", e.stringArray(srcinfo.Conflicts))
	tbl.RawSetString("replaces", e.stringArray(srcinfo.Replaces))

	return tbl
}

func (e *Engine) stringArray(values []string) *glua.LTable {
	tbl := e.L.NewTable()
	for _, value := range values {
		tbl.Append(glua.LString(value))
	}

	return tbl
}
