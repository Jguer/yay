// Package lua loads yay's optional init.lua configuration.
package lua

import (
	"fmt"
	"reflect"

	"github.com/Jguer/yay/v12/pkg/text"

	lua "github.com/yuin/gopher-lua"
)

const (
	globalName   = "yay"
	optTableName = "opt"
)

type Engine struct {
	L        *lua.LState
	autocmds map[string][]Autocmd
	logger   *text.Logger
}

func New() *Engine {
	return NewWithLogger(nil)
}

func NewWithLogger(logger *text.Logger) *Engine {
	state := lua.NewState()
	engine := &Engine{
		L:        state,
		autocmds: make(map[string][]Autocmd),
		logger:   logger,
	}

	yayTbl := state.NewTable()
	state.SetGlobal(globalName, yayTbl)
	state.SetField(yayTbl, optTableName, state.NewTable())
	state.SetField(yayTbl, "abort", state.NewFunction(abort))
	state.SetField(yayTbl, "create_autocmd", state.NewFunction(engine.createAutocmd))
	engine.registerLog(yayTbl)

	return engine
}

func (e *Engine) SetLogger(logger *text.Logger) {
	e.logger = logger
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
