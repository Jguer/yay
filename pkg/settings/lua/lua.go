// Package lua provides an experimental Lua-based configuration surface for
// yay, mirroring neovim's `init.lua` pattern.
//
// The user writes a script at $XDG_CONFIG_HOME/yay/init.lua. The script is run
// inside a fresh gopher-lua state where a global `yay` table is pre-installed:
//
//	yay.opt.<key>          -- setter proxy; values are collected and applied
//	                          to a *settings.Configuration via reflection over
//	                          its `ini:"..."` struct tags. Both snake_case and
//	                          PascalCase keys are accepted (case-insensitive).
//	yay.hook.on_prompt     -- function(name, default) -> string. When set, it
//	                          is invoked by menu code in lieu of the default
//	                          answer. Returning nil/false falls back to the
//	                          default.
//
// Apply takes a generic struct pointer and uses reflection so normal config
// fields do not need bespoke Lua bindings.
package lua

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/Jguer/yay/v12/pkg/settings"
	lua "github.com/yuin/gopher-lua"
)

const (
	globalName                 = "yay"
	optTableName               = "opt"
	apiTableName               = "api"
	hookTableName              = "hook"
	providerTableName          = "provider"
	hookOnPrompt               = "on_prompt"
	hookShouldIncludeAURUpdate = "should_include_aur_update"
	providerListFunctionName   = "list"
	providerSearchFunctionName = "search"
	providerInstallFunctionName = "install"
	providerRunFunctionName    = "upgrade"
)

// Engine wraps a gopher-lua state and serializes access to it.
type Engine struct {
	mu          sync.Mutex
	L           *lua.LState
	callContext context.Context
}

// New creates a new Lua engine with the `yay` global installed.
func New() *Engine {
	L := lua.NewState()

	yayTbl := L.NewTable()
	L.SetGlobal(globalName, yayTbl)

	// yay.opt: plain table; values are read after the script runs.
	L.SetField(yayTbl, optTableName, L.NewTable())
	// yay.api: helper functions exposed to init.lua and runtime hooks.
	L.SetField(yayTbl, apiTableName, L.NewTable())
	// yay.hook: plain table; functions stored here are invoked from Go.
	L.SetField(yayTbl, hookTableName, L.NewTable())
	// yay.provider: registry of custom upgrade providers managed by Lua.
	L.SetField(yayTbl, providerTableName, L.NewTable())

	return &Engine{L: L}
}

func (e *Engine) contextOrBackground() context.Context {
	if e == nil || e.callContext == nil {
		return context.Background()
	}

	return e.callContext
}

// SetAPI registers a helper function under yay.api.<name>.
func (e *Engine) SetAPI(name string, fn lua.LGFunction) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	yayTbl, ok := e.L.GetGlobal(globalName).(*lua.LTable)
	if !ok {
		return fmt.Errorf("lua: missing %s table", globalName)
	}
	apiTbl, ok := e.L.GetField(yayTbl, apiTableName).(*lua.LTable)
	if !ok {
		return fmt.Errorf("lua: missing %s.%s table", globalName, apiTableName)
	}

	e.L.SetField(apiTbl, name, e.L.NewFunction(fn))
	return nil
}

// Close releases the Lua state.
func (e *Engine) Close() {
	if e == nil || e.L == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.L.Close()
	e.L = nil
}

// Run executes the Lua file at path.
func (e *Engine) Run(path string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.L.DoFile(path); err != nil {
		return fmt.Errorf("lua: error running %s: %w", path, err)
	}
	return nil
}

// RunString executes an inline Lua snippet. Primarily for tests.
func (e *Engine) RunString(src string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.L.DoString(src); err != nil {
		return fmt.Errorf("lua: error running snippet: %w", err)
	}
	return nil
}

// Apply walks the `yay.opt` table and writes recognized keys into cfg via
// reflection. cfg must be a pointer to a struct whose fields are tagged with
// `ini:"<name>"`. Unknown keys are returned via the unknown slice (caller can
// log them); type-mismatches are reported in errs.
func (e *Engine) Apply(cfg any) (unknown []string, errs []error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	v := reflect.ValueOf(cfg)
	if v.Kind() != reflect.Pointer || v.Elem().Kind() != reflect.Struct {
		return nil, []error{fmt.Errorf("lua: Apply: expected pointer to struct, got %T", cfg)}
	}
	sv := v.Elem()
	st := sv.Type()

	// Build a case-insensitive index of ini tag -> field index.
	index := make(map[string]int, st.NumField())
	for i := 0; i < st.NumField(); i++ {
		tag := st.Field(i).Tag.Get("ini")
		if tag == "" || tag == "-" {
			continue
		}
		index[strings.ToLower(tag)] = i
	}

	yayTbl, ok := e.L.GetGlobal(globalName).(*lua.LTable)
	if !ok {
		return nil, nil
	}
	optTbl, ok := e.L.GetField(yayTbl, optTableName).(*lua.LTable)
	if !ok {
		return nil, nil
	}

	optTbl.ForEach(func(k, val lua.LValue) {
		key, ok := k.(lua.LString)
		if !ok {
			return
		}
		canonical := strings.ToLower(strings.ReplaceAll(string(key), "_", ""))
		fieldIdx, found := index[canonical]
		if !found {
			unknown = append(unknown, string(key))
			return
		}
		if err := assign(sv.Field(fieldIdx), val); err != nil {
			errs = append(errs, fmt.Errorf("yay.opt.%s: %w", string(key), err))
		}
	})

	return unknown, errs
}

// assign writes a Lua value into a reflect.Value, performing basic coercion.
func assign(field reflect.Value, val lua.LValue) error {
	if !field.CanSet() {
		return fmt.Errorf("field is not settable")
	}
	switch field.Kind() {
	case reflect.String:
		s, ok := val.(lua.LString)
		if !ok {
			return fmt.Errorf("expected string, got %s", val.Type())
		}
		field.SetString(string(s))
	case reflect.Bool:
		b, ok := val.(lua.LBool)
		if !ok {
			return fmt.Errorf("expected boolean, got %s", val.Type())
		}
		field.SetBool(bool(b))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, ok := val.(lua.LNumber)
		if !ok {
			return fmt.Errorf("expected number, got %s", val.Type())
		}
		field.SetInt(int64(n))
	default:
		return fmt.Errorf("unsupported field kind %s", field.Kind())
	}
	return nil
}

// CallOnPrompt invokes yay.hook.on_prompt(name, defaultAns) if it is a
// function. The boolean return is false when the hook is not set, when it
// returns nil/false, or when it returns a non-string value. Errors from the
// Lua call are returned untouched.
func (e *Engine) CallOnPrompt(name, defaultAns string) (string, bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	yayTbl, ok := e.L.GetGlobal(globalName).(*lua.LTable)
	if !ok {
		return "", false, nil
	}
	hookTbl, ok := e.L.GetField(yayTbl, hookTableName).(*lua.LTable)
	if !ok {
		return "", false, nil
	}
	fn, ok := e.L.GetField(hookTbl, hookOnPrompt).(*lua.LFunction)
	if !ok {
		return "", false, nil
	}

	if err := e.L.CallByParam(lua.P{
		Fn:      fn,
		NRet:    1,
		Protect: true,
	}, lua.LString(name), lua.LString(defaultAns)); err != nil {
		return "", false, fmt.Errorf("lua: on_prompt: %w", err)
	}
	ret := e.L.Get(-1)
	e.L.Pop(1)

	switch v := ret.(type) {
	case lua.LString:
		return string(v), true, nil
	case *lua.LNilType:
		return "", false, nil
	case lua.LBool:
		if !bool(v) {
			return "", false, nil
		}
	}
	return "", false, nil
}

// CallShouldIncludeAURUpdate invokes yay.hook.should_include_aur_update(candidate)
// if it is a function. The second return value is false when the hook is not
// set or returns nil.
func (e *Engine) CallShouldIncludeAURUpdate(candidate settings.AURUpdateContext) (bool, bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	yayTbl, ok := e.L.GetGlobal(globalName).(*lua.LTable)
	if !ok {
		return false, false, nil
	}
	hookTbl, ok := e.L.GetField(yayTbl, hookTableName).(*lua.LTable)
	if !ok {
		return false, false, nil
	}
	fn, ok := e.L.GetField(hookTbl, hookShouldIncludeAURUpdate).(*lua.LFunction)
	if !ok {
		return false, false, nil
	}

	arg := e.L.NewTable()
	e.L.SetField(arg, "name", lua.LString(candidate.Name))
	e.L.SetField(arg, "base", lua.LString(candidate.Base))
	e.L.SetField(arg, "repository", lua.LString(candidate.Repository))
	e.L.SetField(arg, "local_version", lua.LString(candidate.LocalVersion))
	e.L.SetField(arg, "remote_version", lua.LString(candidate.RemoteVersion))
	e.L.SetField(arg, "local_build_date", lua.LNumber(candidate.LocalBuildDate))
	e.L.SetField(arg, "remote_last_modified", lua.LNumber(candidate.RemoteLastModified))
	e.L.SetField(arg, "default_include", lua.LBool(candidate.DefaultInclude))

	if err := e.L.CallByParam(lua.P{
		Fn:      fn,
		NRet:    1,
		Protect: true,
	}, arg); err != nil {
		return false, false, fmt.Errorf("lua: should_include_aur_update: %w", err)
	}
	ret := e.L.Get(-1)
	e.L.Pop(1)

	switch v := ret.(type) {
	case lua.LBool:
		return bool(v), true, nil
	case *lua.LNilType:
		return false, false, nil
	default:
		return false, false, fmt.Errorf("lua: should_include_aur_update: expected boolean or nil, got %s", ret.Type())
	}
}

// CallListExternalUpgrades invokes all registered yay.provider.<name>.list
// functions and merges their returned upgrades into a single slice.
func (e *Engine) CallListExternalUpgrades(ctx context.Context) ([]settings.ExternalUpgrade, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	previousCtx := e.callContext
	e.callContext = ctx
	defer func() { e.callContext = previousCtx }()

	providers, err := e.providersTable()
	if err != nil || providers == nil {
		return nil, err
	}
	providerNames, providerTables, err := e.collectProviders(providers)
	if err != nil {
		return nil, err
	}

	upgrades := make([]settings.ExternalUpgrade, 0)
	for _, providerName := range providerNames {
		provider := providerTables[providerName]
		fn, ok := e.L.GetField(provider, providerListFunctionName).(*lua.LFunction)
		if !ok {
			continue
		}

		if err := e.L.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true}); err != nil {
			return nil, fmt.Errorf("lua: provider %s list: %w", providerName, err)
		}

		ret := e.L.Get(-1)
		e.L.Pop(1)

		if ret == lua.LNil {
			continue
		}

		items, ok := ret.(*lua.LTable)
		if !ok {
			return nil, fmt.Errorf("lua: provider %s list: expected table or nil, got %s", providerName, ret.Type())
		}

		providerUpgrades, err := e.externalUpgradesFromLua(providerName, items)
		if err != nil {
			return nil, err
		}

		upgrades = append(upgrades, providerUpgrades...)
	}

	return upgrades, nil
}

// CallRunExternalUpgrades invokes yay.provider.<name>.upgrade for the selected
// upgrades. The boolean return is false when the provider or hook is absent.
func (e *Engine) CallRunExternalUpgrades(ctx context.Context, repository string, upgrades []settings.ExternalUpgrade) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	previousCtx := e.callContext
	e.callContext = ctx
	defer func() { e.callContext = previousCtx }()

	providers, err := e.providersTable()
	if err != nil || providers == nil {
		return false, err
	}

	provider, ok := e.L.GetField(providers, repository).(*lua.LTable)
	if !ok {
		return false, nil
	}

	fn, ok := e.L.GetField(provider, providerRunFunctionName).(*lua.LFunction)
	if !ok {
		return false, nil
	}

	if err := e.L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}, e.externalUpgradesToLua(upgrades)); err != nil {
		return true, fmt.Errorf("lua: provider %s upgrade: %w", repository, err)
	}

	return true, nil
}

// CallSearchExternalPackages invokes all registered yay.provider.<name>.search
// functions and merges their returned results into a single slice.
func (e *Engine) CallSearchExternalPackages(ctx context.Context, terms []string) ([]settings.ExternalSearchResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	previousCtx := e.callContext
	e.callContext = ctx
	defer func() { e.callContext = previousCtx }()

	providers, err := e.providersTable()
	if err != nil || providers == nil {
		return nil, err
	}
	providerNames, providerTables, err := e.collectProviders(providers)
	if err != nil {
		return nil, err
	}

	termsTable := e.L.NewTable()
	for _, term := range terms {
		termsTable.Append(lua.LString(term))
	}

	results := make([]settings.ExternalSearchResult, 0)
	for _, providerName := range providerNames {
		provider := providerTables[providerName]
		fn, ok := e.L.GetField(provider, providerSearchFunctionName).(*lua.LFunction)
		if !ok {
			continue
		}

		if err := e.L.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true}, termsTable); err != nil {
			return nil, fmt.Errorf("lua: provider %s search: %w", providerName, err)
		}

		ret := e.L.Get(-1)
		e.L.Pop(1)
		if ret == lua.LNil {
			continue
		}

		items, ok := ret.(*lua.LTable)
		if !ok {
			return nil, fmt.Errorf("lua: provider %s search: expected table or nil, got %s", providerName, ret.Type())
		}

		providerResults, err := e.externalSearchResultsFromLua(providerName, items)
		if err != nil {
			return nil, err
		}
		results = append(results, providerResults...)
	}

	return results, nil
}

// CallInstallExternalPackages invokes yay.provider.<name>.install for the
// selected targets. The boolean return is false when the provider or hook is
// absent.
func (e *Engine) CallInstallExternalPackages(ctx context.Context, repository string, targets []settings.ExternalInstallTarget) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	previousCtx := e.callContext
	e.callContext = ctx
	defer func() { e.callContext = previousCtx }()

	providers, err := e.providersTable()
	if err != nil || providers == nil {
		return false, err
	}

	provider, ok := e.L.GetField(providers, repository).(*lua.LTable)
	if !ok {
		return false, nil
	}

	fn, ok := e.L.GetField(provider, providerInstallFunctionName).(*lua.LFunction)
	if !ok {
		return false, nil
	}

	if err := e.L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}, e.externalInstallTargetsToLua(targets)); err != nil {
		return true, fmt.Errorf("lua: provider %s install: %w", repository, err)
	}

	return true, nil
}

func (e *Engine) HasProvider(name string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	providers, err := e.providersTable()
	if err != nil || providers == nil {
		return false
	}

	_, ok := e.L.GetField(providers, name).(*lua.LTable)
	return ok
}

func (e *Engine) providersTable() (*lua.LTable, error) {
	yayTbl, ok := e.L.GetGlobal(globalName).(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("lua: missing %s table", globalName)
	}

	providers, ok := e.L.GetField(yayTbl, providerTableName).(*lua.LTable)
	if !ok {
		return nil, nil
	}

	return providers, nil
}

func (e *Engine) collectProviders(providers *lua.LTable) ([]string, map[string]*lua.LTable, error) {
	providerNames := make([]string, 0)
	providerTables := make(map[string]*lua.LTable)
	providers.ForEach(func(k, v lua.LValue) {
		name, ok := k.(lua.LString)
		if !ok {
			return
		}
		provider, ok := v.(*lua.LTable)
		if !ok {
			return
		}
		providerName := string(name)
		providerNames = append(providerNames, providerName)
		providerTables[providerName] = provider
	})
	sort.Strings(providerNames)

	return providerNames, providerTables, nil
}

func (e *Engine) externalUpgradesFromLua(providerName string, items *lua.LTable) ([]settings.ExternalUpgrade, error) {
	upgrades := make([]settings.ExternalUpgrade, 0)
	items.ForEach(func(_ lua.LValue, value lua.LValue) {
		if value == lua.LNil {
			return
		}
		item, ok := value.(*lua.LTable)
		if !ok {
			upgrades = append(upgrades, settings.ExternalUpgrade{Repository: providerName, Extra: fmt.Sprintf("__error__:%s", value.Type())})
			return
		}

		upgrade := settings.ExternalUpgrade{
			Name:          luaTableString(item, "name"),
			Base:          luaTableString(item, "base"),
			Repository:    luaTableString(item, "repository"),
			LocalVersion:  luaTableString(item, "local_version"),
			RemoteVersion: luaTableString(item, "remote_version"),
			Extra:         luaTableString(item, "extra"),
		}
		if upgrade.Repository == "" {
			upgrade.Repository = providerName
		}
		upgrades = append(upgrades, upgrade)
	})

	for _, upgrade := range upgrades {
		if strings.HasPrefix(upgrade.Extra, "__error__:") {
			return nil, fmt.Errorf("lua: provider %s list: expected table items, got %s", providerName, strings.TrimPrefix(upgrade.Extra, "__error__:"))
		}
		if upgrade.Name == "" {
			return nil, fmt.Errorf("lua: provider %s list: upgrade entry missing name", providerName)
		}
	}

	return upgrades, nil
}

func (e *Engine) externalUpgradesToLua(upgrades []settings.ExternalUpgrade) *lua.LTable {
	tbl := e.L.NewTable()
	for _, upgrade := range upgrades {
		item := e.L.NewTable()
		e.L.SetField(item, "name", lua.LString(upgrade.Name))
		e.L.SetField(item, "base", lua.LString(upgrade.Base))
		e.L.SetField(item, "repository", lua.LString(upgrade.Repository))
		e.L.SetField(item, "local_version", lua.LString(upgrade.LocalVersion))
		e.L.SetField(item, "remote_version", lua.LString(upgrade.RemoteVersion))
		e.L.SetField(item, "extra", lua.LString(upgrade.Extra))
		tbl.Append(item)
	}

	return tbl
}

func (e *Engine) externalSearchResultsFromLua(providerName string, items *lua.LTable) ([]settings.ExternalSearchResult, error) {
	results := make([]settings.ExternalSearchResult, 0)
	items.ForEach(func(_ lua.LValue, value lua.LValue) {
		if value == lua.LNil {
			return
		}
		item, ok := value.(*lua.LTable)
		if !ok {
			results = append(results, settings.ExternalSearchResult{Repository: providerName, Extra: fmt.Sprintf("__error__:%s", value.Type())})
			return
		}

		result := settings.ExternalSearchResult{
			Name:             luaTableString(item, "name"),
			Base:             luaTableString(item, "base"),
			Repository:       luaTableString(item, "repository"),
			Version:          luaTableString(item, "version"),
			InstalledVersion: luaTableString(item, "installed_version"),
			Description:      luaTableString(item, "description"),
			Extra:            luaTableString(item, "extra"),
		}
		if result.Repository == "" {
			result.Repository = providerName
		}
		results = append(results, result)
	})

	for _, result := range results {
		if strings.HasPrefix(result.Extra, "__error__:") {
			return nil, fmt.Errorf("lua: provider %s search: expected table items, got %s", providerName, strings.TrimPrefix(result.Extra, "__error__:"))
		}
		if result.Name == "" {
			return nil, fmt.Errorf("lua: provider %s search: result entry missing name", providerName)
		}
	}

	return results, nil
}

func (e *Engine) externalInstallTargetsToLua(targets []settings.ExternalInstallTarget) *lua.LTable {
	tbl := e.L.NewTable()
	for _, target := range targets {
		item := e.L.NewTable()
		e.L.SetField(item, "name", lua.LString(target.Name))
		e.L.SetField(item, "repository", lua.LString(target.Repository))
		tbl.Append(item)
	}

	return tbl
}

func luaTableString(tbl *lua.LTable, field string) string {
	value := tbl.RawGetString(field)
	if value == lua.LNil {
		return ""
	}
	if str, ok := value.(lua.LString); ok {
		return string(str)
	}

	return fmt.Sprint(value)
}
