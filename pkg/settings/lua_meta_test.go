//go:build !integration

package settings

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var luaMetaFieldRe = regexp.MustCompile(`^---@field\s+([A-Za-z_][A-Za-z0-9_]*)`)

// luaMetaOptFields returns the field names declared on the yay.opt class in
// meta/yay.d.lua.
func luaMetaOptFields(meta string) map[string]bool {
	fields := map[string]bool{}

	inOpt := false

	for _, line := range strings.Split(meta, "\n") {
		switch {
		case strings.HasPrefix(line, "---@class "):
			inOpt = strings.TrimSpace(strings.TrimPrefix(line, "---@class ")) == "yay.opt"
		case inOpt:
			if m := luaMetaFieldRe.FindStringSubmatch(line); m != nil {
				fields[m[1]] = true
			}
		}
	}

	return fields
}

// TestLuaMetaCoversAllOptions guards meta/yay.d.lua against drifting from the
// lua-tagged fields of Configuration: lua-language-server users would otherwise
// lose completion and type checking for newly added options.
func TestLuaMetaCoversAllOptions(t *testing.T) {
	meta, err := os.ReadFile(filepath.Join("..", "..", "meta", "yay.d.lua"))
	require.NoError(t, err)

	metaFields := luaMetaOptFields(string(meta))
	require.NotEmpty(t, metaFields)

	configFields := map[string]bool{}

	typ := reflect.TypeOf(Configuration{})
	for i := range typ.NumField() {
		if name := typ.Field(i).Tag.Get("lua"); name != "" && name != "-" {
			configFields[name] = true
		}
	}

	for name := range configFields {
		assert.Contains(t, metaFields, name, "meta/yay.d.lua is missing yay.opt field %q", name)
	}

	for name := range metaFields {
		assert.True(t, configFields[name], "meta/yay.d.lua declares unknown yay.opt field %q", name)
	}
}

func TestLuaMetaOptFields(t *testing.T) {
	meta := "---@class yay.opt\n---@field a string\n---@field b string\n\n---@class yay.log\n---@field c string\n"
	assert.Equal(t, map[string]bool{"a": true, "b": true}, luaMetaOptFields(meta))
}
