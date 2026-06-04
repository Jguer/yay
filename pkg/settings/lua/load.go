package lua

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/text"
	lua "github.com/yuin/gopher-lua"
)

func registerBuiltins(e *Engine, logger *text.Logger) error {
	bindings := map[string]lua.LGFunction{
		"getenv": func(L *lua.LState) int {
			key := L.CheckString(1)
			value, ok := os.LookupEnv(key)
			if !ok {
				L.Push(lua.LNil)
				return 1
			}
			L.Push(lua.LString(value))
			return 1
		},
		"expand": func(L *lua.LState) int {
			value := os.ExpandEnv(L.CheckString(1))
			if len(value) >= 2 && value[:2] == "~/" {
				value = filepath.Join(os.Getenv("HOME"), value[2:])
			}
			L.Push(lua.LString(value))
			return 1
		},
		"info": func(L *lua.LState) int {
			if logger != nil {
				logger.Infoln(L.CheckString(1))
			}
			return 0
		},
		"warn": func(L *lua.LState) int {
			if logger != nil {
				logger.Warnln(L.CheckString(1))
			}
			return 0
		},
		"error": func(L *lua.LState) int {
			if logger != nil {
				logger.Errorln(L.CheckString(1))
			}
			return 0
		},
		"capture": func(L *lua.LState) int {
			args := luaCommandArgs(L)
			cmd := exec.CommandContext(e.contextOrBackground(), args[0], args[1:]...)
			stdout, stderr, exitCode := runCapture(cmd)
			L.Push(lua.LString(stdout))
			L.Push(lua.LString(stderr))
			L.Push(lua.LNumber(exitCode))
			return 3
		},
		"run": func(L *lua.LState) int {
			args := luaCommandArgs(L)
			cmd := exec.CommandContext(e.contextOrBackground(), args[0], args[1:]...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err := cmd.Run()
			L.Push(lua.LNumber(commandExitCode(err)))
			return 1
		},
		"json_decode": func(L *lua.LState) int {
			input := L.CheckString(1)
			var decoded any
			if err := json.Unmarshal([]byte(input), &decoded); err != nil {
				L.RaiseError("json_decode: %v", err)
				return 0
			}
			L.Push(toLuaValue(L, decoded))
			return 1
		},
	}

	for name, fn := range bindings {
		if err := e.SetAPI(name, fn); err != nil {
			return err
		}
	}

	return nil
}

func luaCommandArgs(L *lua.LState) []string {
	args := make([]string, 0, max(L.GetTop(), 1))
	if tbl, ok := L.Get(1).(*lua.LTable); ok {
		tbl.ForEach(func(_ lua.LValue, value lua.LValue) {
			args = append(args, value.String())
		})
	} else {
		for i := 1; i <= L.GetTop(); i++ {
			args = append(args, L.CheckString(i))
		}
	}

	if len(args) == 0 {
		L.ArgError(1, "expected command")
	}

	return args
}

func runCapture(cmd *exec.Cmd) (stdout, stderr string, code int) {
	output, err := cmd.Output()
	stdout = string(output)
	if err == nil {
		return stdout, "", 0
	}

	code = commandExitCode(err)
	exitErr := &exec.ExitError{}
	if errors.As(err, &exitErr) {
		stderr = string(exitErr.Stderr)
		return stdout, stderr, code
	}

	return stdout, err.Error(), code
}

func commandExitCode(err error) int {
	if err == nil {
		return 0
	}

	exitErr := &exec.ExitError{}
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}

	return -1
}

func toLuaValue(L *lua.LState, value any) lua.LValue {
	switch v := value.(type) {
	case nil:
		return lua.LNil
	case string:
		return lua.LString(v)
	case bool:
		return lua.LBool(v)
	case float64:
		return lua.LNumber(v)
	case []any:
		tbl := L.NewTable()
		for _, item := range v {
			tbl.Append(toLuaValue(L, item))
		}
		return tbl
	case map[string]any:
		tbl := L.NewTable()
		for key, item := range v {
			L.SetField(tbl, key, toLuaValue(L, item))
		}
		return tbl
	default:
		return lua.LString(fmt.Sprint(v))
	}
}

// LoadInto runs the user's init.lua (if present) and applies its `yay.opt`
// table onto cfg, then attaches the engine to cfg so hooks can be invoked at
// runtime. If no init.lua is present, LoadInto is a no-op and returns nil.
//
// Errors from Apply (type mismatches) and unknown keys are reported via
// logger but do not fail the load.
func LoadInto(logger *text.Logger, cfg *settings.Configuration) error {
	path, err := settings.ResolveLuaConfigPath()
	if err != nil {
		return fmt.Errorf("lua: resolve init.lua: %w", err)
	}
	if path == "" {
		return nil
	}

	e := New()
	if err := registerBuiltins(e, logger); err != nil {
		e.Close()
		return err
	}
	if err := e.Run(path); err != nil {
		e.Close()
		return err
	}

	unknown, errs := e.Apply(cfg)
	if logger != nil {
		for _, k := range unknown {
			logger.Warnln("lua: unknown yay.opt key:", k)
		}
		for _, err := range errs {
			logger.Errorln(err)
		}
	}

	cfg.SetLuaEngine(e)
	return nil
}
