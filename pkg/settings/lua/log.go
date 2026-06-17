package lua

import (
	"github.com/Jguer/yay/v13/pkg/text"

	glua "github.com/yuin/gopher-lua"
)

const logTableName = "log"

func (e *Engine) registerLog(yayTbl *glua.LTable) {
	logTbl := e.L.NewTable()
	e.L.SetField(logTbl, "debug", e.L.NewFunction(e.newLogFn((*text.Logger).Debugln)))
	e.L.SetField(logTbl, "info", e.L.NewFunction(e.newLogFn((*text.Logger).Infoln)))
	e.L.SetField(logTbl, "warn", e.L.NewFunction(e.newLogFn((*text.Logger).Warnln)))
	e.L.SetField(logTbl, "error", e.L.NewFunction(e.newLogFn((*text.Logger).Errorln)))
	e.L.SetField(yayTbl, logTableName, logTbl)
}

func (e *Engine) newLogFn(method func(*text.Logger, ...any)) glua.LGFunction {
	return func(state *glua.LState) int {
		if e.logger != nil {
			method(e.logger, logArgs(state)...)
		}
		return 0
	}
}

func logArgs(state *glua.LState) []any {
	top := state.GetTop()
	args := make([]any, top)
	for i := 1; i <= top; i++ {
		args[i-1] = state.ToStringMeta(state.Get(i)).String()
	}

	return args
}
