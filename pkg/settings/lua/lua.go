// Package lua loads yay's optional init.lua configuration.
package lua

import (
	"fmt"
	"reflect"

	lua "github.com/yuin/gopher-lua"
)

const (
	globalName   = "yay"
	optTableName = "opt"
)

var validEvents = map[string]bool{"render_search": true}

type Engine struct {
	L             *lua.LState
	renderSearch  *lua.LFunction
}

func New() *Engine {
	state := lua.NewState()

	e := &Engine{L: state}

	yayTbl := state.NewTable()
	state.SetGlobal(globalName, yayTbl)
	state.SetField(yayTbl, optTableName, state.NewTable())
	state.SetField(yayTbl, "on", state.NewFunction(e.luaOn))

	return e
}

// luaOn registers a Lua callback for the render_search event (yay.on).
func (e *Engine) luaOn(L *lua.LState) int {
	event := L.CheckString(1)
	fn := L.CheckFunction(2)

	if !validEvents[event] {
		L.RaiseError("yay.on: unknown event %q", event)
		return 0
	}

	e.renderSearch = fn

	return 0
}

// HasHooks reports whether any search-display callback has been registered.
func (e *Engine) HasHooks() bool {
	return e.renderSearch != nil
}

// RenderSearch invokes the render_search callback with the full result list.
// handled=true when the callback returned a string; false when no callback is
// registered or it deferred (returned nil/nothing/non-string).
func (e *Engine) RenderSearch(results []map[string]any) (string, bool, error) {
	if e.renderSearch == nil {
		return "", false, nil
	}

	e.L.Push(e.renderSearch)
	e.L.Push(e.resultsToTable(results))

	if err := e.L.PCall(1, 1, nil); err != nil {
		return "", false, fmt.Errorf("init.lua render_search hook: %w", err)
	}

	ret := e.L.Get(-1)
	e.L.Pop(1)

	if s, ok := ret.(lua.LString); ok {
		return string(s), true, nil
	}

	return "", false, nil
}

func (e *Engine) resultsToTable(results []map[string]any) *lua.LTable {
	tbl := e.L.NewTable()
	for i := range results {
		tbl.Append(e.toTable(results[i]))
	}

	return tbl
}

// toTable converts a Go map into a Lua table for passing to a callback.
func (e *Engine) toTable(pkg map[string]any) *lua.LTable {
	tbl := e.L.NewTable()

	for k, v := range pkg {
		switch val := v.(type) {
		case string:
			e.L.SetField(tbl, k, lua.LString(val))
		case int:
			e.L.SetField(tbl, k, lua.LNumber(val))
		case int64:
			e.L.SetField(tbl, k, lua.LNumber(val))
		case float64:
			e.L.SetField(tbl, k, lua.LNumber(val))
		case bool:
			e.L.SetField(tbl, k, lua.LBool(val))
		case []string:
			seq := e.L.NewTable()
			for _, item := range val {
				seq.Append(lua.LString(item))
			}
			e.L.SetField(tbl, k, seq)
		}
	}

	return tbl
}

func (e *Engine) Close() {
	e.L.Close()
}

// Apply writes recognized yay.opt values into cfg.
func (e *Engine) Apply(cfg any) (unknown []string, errs []error) {
	v := reflect.ValueOf(cfg)
	if v.Kind() != reflect.Pointer || v.Elem().Kind() != reflect.Struct {
		return nil, []error{fmt.Errorf("lua: Apply expected pointer to struct, got %T", cfg)}
	}

	sv := v.Elem()
	st := sv.Type()

	index := make(map[string]int, st.NumField())

	for i := range st.NumField() {
		field := st.Field(i)
		if name := luaKeyForField(&field); name != "" {
			index[name] = i
		}
	}

	optTbl, ok := e.optTable()
	if !ok {
		return nil, nil
	}

	optTbl.ForEach(func(k, val lua.LValue) {
		key, ok := k.(lua.LString)
		if !ok {
			return
		}

		fieldIdx, found := index[string(key)]
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

func (e *Engine) optTable() (*lua.LTable, bool) {
	yayTbl, ok := e.L.GetGlobal(globalName).(*lua.LTable)
	if !ok {
		return nil, false
	}

	optTbl, ok := e.L.GetField(yayTbl, optTableName).(*lua.LTable)

	return optTbl, ok
}

func luaKeyForField(field *reflect.StructField) string {
	name := field.Tag.Get("lua")
	if name != "" && name != "-" {
		return name
	}

	return ""
}

func assign(field reflect.Value, val lua.LValue) error {
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
	case reflect.Int, reflect.Int64:
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
