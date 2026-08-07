// Package lua loads yay's optional init.lua configuration.
package lua

import (
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/Jguer/yay/v13/pkg/text"

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
	index := luaFieldIndex(sv.Type())

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

// SetSearchDir makes require() resolve modules relative to dir (e.g.
// require("hooks.maintainer_change")) and drops the default "./?.lua" entry so
// require() searches only dir and the absolute system paths, never an unrelated
// CWD. When dir is the CWD, the prepended patterns already cover it.
func (e *Engine) SetSearchDir(dir string) {
	pkg := e.L.GetGlobal("package")

	current, ok := e.L.GetField(pkg, "path").(lua.LString)
	if !ok {
		return
	}

	pattern := filepath.Join(dir, "?.lua") + ";" + filepath.Join(dir, "?", "init.lua")
	e.L.SetField(pkg, "path", lua.LString(pattern+";"+absolutePatterns(string(current))))
}

// absolutePatterns keeps only the absolute entries of a package.path string,
// dropping CWD-relative ones such as "./?.lua".
func absolutePatterns(path string) string {
	segments := strings.Split(path, ";")
	kept := segments[:0]

	for _, seg := range segments {
		if filepath.IsAbs(seg) {
			kept = append(kept, seg)
		}
	}

	return strings.Join(kept, ";")
}

func luaKeyForField(field *reflect.StructField) string {
	name := field.Tag.Get("lua")
	if name != "" && name != "-" {
		return name
	}

	return ""
}

// luaFieldIndex maps each lua-tagged field name of st to its field index.
func luaFieldIndex(st reflect.Type) map[string]int {
	index := make(map[string]int, st.NumField())

	for i := range st.NumField() {
		field := st.Field(i)
		if name := luaKeyForField(&field); name != "" {
			index[name] = i
		}
	}

	return index
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
	case reflect.Slice:
		return assignStructSlice(field, val)
	default:
		return fmt.Errorf("unsupported field kind %s", field.Kind())
	}

	return nil
}

// assignStructSlice fills a []Struct field from a Lua table keyed by name, e.g.
//
//	{ ["core"] = { url = "..." }, ["extra"] = { url = "..." } }
//
// Each entry becomes one struct: the table key populates the element's
// lua:"name" field and the sub-table populates the remaining fields. Entries
// are sorted by name so the resulting slice is deterministic despite Lua's
// unordered table iteration.
func assignStructSlice(field reflect.Value, val lua.LValue) error {
	elemType := field.Type().Elem()
	if elemType.Kind() != reflect.Struct {
		return fmt.Errorf("unsupported slice element kind %s", elemType.Kind())
	}

	tbl, ok := val.(*lua.LTable)
	if !ok {
		return fmt.Errorf("expected table, got %s", val.Type())
	}

	elemIndex := luaFieldIndex(elemType)

	type namedElem struct {
		name string
		elem reflect.Value
	}

	var (
		entries  []namedElem
		firstErr error
	)

	tbl.ForEach(func(k, entry lua.LValue) {
		if firstErr != nil {
			return
		}

		name, ok := k.(lua.LString)
		if !ok {
			firstErr = fmt.Errorf("entry keys must be strings, got %s", k.Type())
			return
		}

		entryTbl, ok := entry.(*lua.LTable)
		if !ok {
			firstErr = fmt.Errorf("entry %q must be a table, got %s", string(name), entry.Type())
			return
		}

		elem := reflect.New(elemType).Elem()
		if nameIdx, found := elemIndex["name"]; found {
			elem.Field(nameIdx).SetString(string(name))
		}

		if err := assignStructFields(elem, entryTbl, elemIndex); err != nil {
			firstErr = fmt.Errorf("entry %q: %w", string(name), err)
			return
		}

		entries = append(entries, namedElem{name: string(name), elem: elem})
	})

	if firstErr != nil {
		return firstErr
	}

	// Sort by name so the resulting slice is deterministic despite Lua's
	// unordered table iteration.
	slices.SortFunc(entries, func(a, b namedElem) int {
		return strings.Compare(a.name, b.name)
	})

	out := reflect.MakeSlice(field.Type(), len(entries), len(entries))
	for i, entry := range entries {
		out.Index(i).Set(entry.elem)
	}

	field.Set(out)

	return nil
}

// assignStructFields assigns the entries of tbl onto struct value sv, matching
// each key against the lua:"..." tags in index. Unknown keys are errors so
// typos in nested option tables fail fast, mirroring top-level opt handling.
func assignStructFields(sv reflect.Value, tbl *lua.LTable, index map[string]int) error {
	var firstErr error

	tbl.ForEach(func(k, entry lua.LValue) {
		if firstErr != nil {
			return
		}

		key, ok := k.(lua.LString)
		if !ok {
			firstErr = fmt.Errorf("keys must be strings, got %s", k.Type())
			return
		}

		fieldIdx, found := index[string(key)]
		if !found {
			firstErr = fmt.Errorf("unknown key %q", string(key))
			return
		}

		if err := assign(sv.Field(fieldIdx), entry); err != nil {
			firstErr = fmt.Errorf("%s: %w", string(key), err)
		}
	})

	return firstErr
}
