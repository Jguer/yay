package lua

import (
	"fmt"

	mapset "github.com/deckarep/golang-set/v2"
	glua "github.com/yuin/gopher-lua"
)

const (
	EventAURPreInstall   = "AURPreInstall"
	EventAURPostDownload = "AURPostDownload"
	EventUpgradeSelect   = "UpgradeSelect"
	EventPostInstall     = "PostInstall"
	EventSearchFilter    = "SearchFilter"
)

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

type UpgradeSelectEvent struct {
	Upgrades           []UpgradeSelectPackage
	PulledDependencies []UpgradeSelectPackage
}

type UpgradeSelectPackage struct {
	ID            int
	Name          string
	Base          string
	Repository    string
	LocalVersion  string
	RemoteVersion string
	Reason        string
	LastModified  int64
	Maintainer    string
}

type UpgradeSelectResult struct {
	Exclude  []string
	SkipMenu bool
}

type PostInstallEvent struct {
	Packages []PostInstallPackage
}

type PostInstallPackage struct {
	Name         string
	Version      string
	LocalVersion string
	Source       string
	Reason       string
	Installed    bool
	Upgrade      bool
	Devel        bool
}

type SearchFilterEvent struct {
	Results []SearchResultPackage
}

type SearchResultPackage struct {
	Source         string
	Name           string
	Description    string
	Base           string
	Votes          int
	Popularity     float64
	FirstSubmitted int
	LastModified   int
	Provides       []string
}

type SearchResultRef struct {
	Source string
	Name   string
}

func (e *Engine) createAutocmd(state *glua.LState) int {
	event := state.CheckString(1)
	if event != EventAURPreInstall && event != EventAURPostDownload &&
		event != EventUpgradeSelect && event != EventPostInstall && event != EventSearchFilter {
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
	return e.runAUREvent(EventAURPreInstall, event)
}

func (e *Engine) RunAURPostDownload(event *AURPreInstallEvent) error {
	return e.runAUREvent(EventAURPostDownload, event)
}

func (e *Engine) runAUREvent(eventName string, event *AURPreInstallEvent) error {
	if !e.HasAutocmd(eventName) {
		return nil
	}

	for _, autocmd := range e.autocmds[eventName] {
		if err := e.L.CallByParam(glua.P{
			Fn:      autocmd.callback,
			NRet:    0,
			Protect: true,
		}, e.aurEventTable(eventName, event)); err != nil {
			return fmt.Errorf("%s %s: %w", eventName, event.Base, wrapLuaErr(err))
		}
	}

	return nil
}

func (e *Engine) RunUpgradeSelect(event *UpgradeSelectEvent) (UpgradeSelectResult, error) {
	var result UpgradeSelectResult
	if !e.HasAutocmd(EventUpgradeSelect) {
		return result, nil
	}

	validExcludes := mapset.NewThreadUnsafeSetWithSize[string](len(event.Upgrades))
	for i := range event.Upgrades {
		validExcludes.Add(event.Upgrades[i].Name)
	}

	seenExcludes := mapset.NewThreadUnsafeSet[string]()
	for _, autocmd := range e.autocmds[EventUpgradeSelect] {
		if err := e.L.CallByParam(glua.P{
			Fn:      autocmd.callback,
			NRet:    1,
			Protect: true,
		}, e.upgradeSelectTable(event)); err != nil {
			return result, fmt.Errorf("%s: %w", EventUpgradeSelect, wrapLuaErr(err))
		}

		value := e.L.Get(-1)
		e.L.Pop(1)

		hookResult, err := e.parseUpgradeSelectResult(value, validExcludes)
		if err != nil {
			return result, fmt.Errorf("%s: %w", EventUpgradeSelect, err)
		}

		for _, name := range hookResult.Exclude {
			if !seenExcludes.Add(name) {
				continue
			}
			result.Exclude = append(result.Exclude, name)
		}

		if hookResult.SkipMenu {
			result.SkipMenu = true
		}
	}

	return result, nil
}

func (e *Engine) newEventTable(eventName string) (eventTable, data *glua.LTable) {
	eventTable = e.L.NewTable()
	data = e.L.NewTable()
	eventTable.RawSetString("event", glua.LString(eventName))
	eventTable.RawSetString("data", data)

	return eventTable, data
}

func (e *Engine) aurEventTable(eventName string, event *AURPreInstallEvent) *glua.LTable {
	eventTable, data := e.newEventTable(eventName)
	eventTable.RawSetString("match", glua.LString(event.Base))

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

func (e *Engine) upgradeSelectTable(event *UpgradeSelectEvent) *glua.LTable {
	eventTable, data := e.newEventTable(EventUpgradeSelect)
	data.RawSetString("upgrades", e.upgradeSelectPackagesTable(event.Upgrades))
	data.RawSetString("pulled_dependencies", e.upgradeSelectPackagesTable(event.PulledDependencies))

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

func (e *Engine) upgradeSelectPackagesTable(packages []UpgradeSelectPackage) *glua.LTable {
	state := e.L
	tbl := state.NewTable()

	for i := range packages {
		pkg := &packages[i]
		pkgTbl := state.NewTable()
		pkgTbl.RawSetString("id", glua.LNumber(pkg.ID))
		pkgTbl.RawSetString("name", glua.LString(pkg.Name))
		pkgTbl.RawSetString("base", glua.LString(pkg.Base))
		pkgTbl.RawSetString("repository", glua.LString(pkg.Repository))
		pkgTbl.RawSetString("local_version", glua.LString(pkg.LocalVersion))
		pkgTbl.RawSetString("remote_version", glua.LString(pkg.RemoteVersion))
		pkgTbl.RawSetString("reason", glua.LString(pkg.Reason))
		pkgTbl.RawSetString("last_modified", glua.LNumber(pkg.LastModified))
		pkgTbl.RawSetString("maintainer", glua.LString(pkg.Maintainer))
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

func (e *Engine) parseUpgradeSelectResult(value glua.LValue, validExcludes mapset.Set[string]) (UpgradeSelectResult, error) {
	var result UpgradeSelectResult
	if value == glua.LNil {
		return result, nil
	}

	tbl, ok := value.(*glua.LTable)
	if !ok {
		return result, fmt.Errorf("callback must return nil or table, got %s", value.Type())
	}

	if excludeValue := tbl.RawGetString("exclude"); excludeValue != glua.LNil {
		excludeTbl, ok := excludeValue.(*glua.LTable)
		if !ok {
			return result, fmt.Errorf("exclude must be a table")
		}

		var parseErr error
		excludeTbl.ForEach(func(_ glua.LValue, val glua.LValue) {
			if parseErr != nil {
				return
			}

			lname, ok := val.(glua.LString)
			if !ok {
				parseErr = fmt.Errorf("exclude entries must be strings")
				return
			}

			name := string(lname)
			if !validExcludes.Contains(name) {
				parseErr = fmt.Errorf("unknown upgrade exclusion %q", name)
				return
			}

			result.Exclude = append(result.Exclude, name)
		})
		if parseErr != nil {
			return result, parseErr
		}
	}

	if skipMenuValue := tbl.RawGetString("skip_menu"); skipMenuValue != glua.LNil {
		skipMenu, ok := skipMenuValue.(glua.LBool)
		if !ok {
			return result, fmt.Errorf("skip_menu must be a boolean")
		}

		result.SkipMenu = bool(skipMenu)
	}

	return result, nil
}

func (e *Engine) stringArray(values []string) *glua.LTable {
	tbl := e.L.NewTable()
	for _, value := range values {
		tbl.Append(glua.LString(value))
	}

	return tbl
}

func (e *Engine) RunPostInstall(event *PostInstallEvent) error {
	if !e.HasAutocmd(EventPostInstall) {
		return nil
	}

	for _, autocmd := range e.autocmds[EventPostInstall] {
		if err := e.L.CallByParam(glua.P{
			Fn:      autocmd.callback,
			NRet:    0,
			Protect: true,
		}, e.postInstallTable(event)); err != nil {
			return fmt.Errorf("%s: %w", EventPostInstall, wrapLuaErr(err))
		}
	}

	return nil
}

func (e *Engine) RunSearchFilter(event *SearchFilterEvent) ([]SearchResultRef, error) {
	if !e.HasAutocmd(EventSearchFilter) {
		return nil, nil
	}

	active := event.Results

	for _, autocmd := range e.autocmds[EventSearchFilter] {
		// Rebuild the validation map from the current active set so that a
		// later hook cannot reintroduce packages that an earlier hook dropped.
		activeByRef := make(map[SearchResultRef]int, len(active))
		for i, pkg := range active {
			activeByRef[SearchResultRef{Source: pkg.Source, Name: pkg.Name}] = i
		}

		if err := e.L.CallByParam(glua.P{
			Fn:      autocmd.callback,
			NRet:    1,
			Protect: true,
		}, e.searchFilterTable(active)); err != nil {
			return nil, fmt.Errorf("%s: %w", EventSearchFilter, wrapLuaErr(err))
		}

		value := e.L.Get(-1)
		e.L.Pop(1)

		refs, returned, err := parseSearchFilterResult(value, activeByRef)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", EventSearchFilter, err)
		}

		if !returned {
			continue
		}

		next := make([]SearchResultPackage, 0, len(refs))
		for _, ref := range refs {
			next = append(next, active[activeByRef[ref]])
		}

		active = next
	}

	result := make([]SearchResultRef, len(active))
	for i, pkg := range active {
		result[i] = SearchResultRef{Source: pkg.Source, Name: pkg.Name}
	}

	return result, nil
}

func (e *Engine) postInstallTable(event *PostInstallEvent) *glua.LTable {
	eventTable, data := e.newEventTable(EventPostInstall)
	data.RawSetString("packages", e.postInstallPackagesTable(event.Packages))

	return eventTable
}

func (e *Engine) postInstallPackagesTable(packages []PostInstallPackage) *glua.LTable {
	tbl := e.L.NewTable()

	for i := range packages {
		pkg := &packages[i]
		pkgTbl := e.L.NewTable()
		pkgTbl.RawSetString("name", glua.LString(pkg.Name))
		pkgTbl.RawSetString("version", glua.LString(pkg.Version))
		pkgTbl.RawSetString("local_version", glua.LString(pkg.LocalVersion))
		pkgTbl.RawSetString("source", glua.LString(pkg.Source))
		pkgTbl.RawSetString("reason", glua.LString(pkg.Reason))
		pkgTbl.RawSetString("installed", glua.LBool(pkg.Installed))
		pkgTbl.RawSetString("upgrade", glua.LBool(pkg.Upgrade))
		pkgTbl.RawSetString("devel", glua.LBool(pkg.Devel))
		tbl.Append(pkgTbl)
	}

	return tbl
}

func (e *Engine) searchFilterTable(packages []SearchResultPackage) *glua.LTable {
	eventTable, data := e.newEventTable(EventSearchFilter)
	data.RawSetString("results", e.searchResultPackagesTable(packages))

	return eventTable
}

func (e *Engine) searchResultPackagesTable(packages []SearchResultPackage) *glua.LTable {
	tbl := e.L.NewTable()

	for i := range packages {
		pkg := &packages[i]
		pkgTbl := e.L.NewTable()
		pkgTbl.RawSetString("source", glua.LString(pkg.Source))
		pkgTbl.RawSetString("name", glua.LString(pkg.Name))
		pkgTbl.RawSetString("description", glua.LString(pkg.Description))
		pkgTbl.RawSetString("base", glua.LString(pkg.Base))
		pkgTbl.RawSetString("votes", glua.LNumber(pkg.Votes))
		pkgTbl.RawSetString("popularity", glua.LNumber(pkg.Popularity))
		pkgTbl.RawSetString("first_submitted", glua.LNumber(pkg.FirstSubmitted))
		pkgTbl.RawSetString("last_modified", glua.LNumber(pkg.LastModified))
		pkgTbl.RawSetString("provides", e.stringArray(pkg.Provides))
		tbl.Append(pkgTbl)
	}

	return tbl
}

func parseSearchFilterResult(value glua.LValue, valid map[SearchResultRef]int) ([]SearchResultRef, bool, error) {
	if value == glua.LNil {
		return nil, false, nil
	}

	tbl, ok := value.(*glua.LTable)
	if !ok {
		return nil, false, fmt.Errorf("callback must return nil or a table, got %s", value.Type())
	}

	var (
		refs     []SearchResultRef
		parseErr error
	)

	seen := mapset.NewThreadUnsafeSet[SearchResultRef]()

	tbl.ForEach(func(_ glua.LValue, val glua.LValue) {
		if parseErr != nil {
			return
		}

		entry, ok := val.(*glua.LTable)
		if !ok {
			parseErr = fmt.Errorf("each result must be a table")
			return
		}

		source, ok := entry.RawGetString("source").(glua.LString)
		if !ok {
			parseErr = fmt.Errorf("result source must be a string")
			return
		}

		name, ok := entry.RawGetString("name").(glua.LString)
		if !ok {
			parseErr = fmt.Errorf("result name must be a string")
			return
		}

		ref := SearchResultRef{Source: string(source), Name: string(name)}
		if _, exists := valid[ref]; !exists {
			parseErr = fmt.Errorf("unknown search result %s/%s", ref.Source, ref.Name)
			return
		}

		if !seen.Add(ref) {
			return
		}

		refs = append(refs, ref)
	})

	if parseErr != nil {
		return nil, false, parseErr
	}

	return refs, true, nil
}
