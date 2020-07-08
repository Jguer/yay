package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOption_Add(t *testing.T) {
	type fields struct {
		Args []string
	}
	type args struct {
		arg string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []string
	}{
		{name: "simple add", fields: fields{
			Args: []string{"a", "b"},
		}, args: args{
			arg: "c",
		}, want: []string{"a", "b", "c"}},
		{name: "null add", fields: fields{
			Args: nil,
		}, args: args{
			arg: "c",
		}, want: []string{"c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &Option{
				Args: tt.fields.Args,
			}
			o.Add(tt.args.arg)
			assert.ElementsMatch(t, tt.want, o.Args)
		})
	}
}

func TestOption_Set(t *testing.T) {
	type fields struct {
		Args []string
	}
	type args struct {
		arg string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []string
	}{
		{name: "simple set", fields: fields{
			Args: []string{"a", "b"},
		}, args: args{
			arg: "c",
		}, want: []string{"c"}},
		{name: "null set", fields: fields{
			Args: nil,
		}, args: args{
			arg: "c",
		}, want: []string{"c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &Option{
				Args: tt.fields.Args,
			}
			o.Set(tt.args.arg)
			assert.ElementsMatch(t, tt.want, o.Args)
		})
	}
}

func TestOption_First(t *testing.T) {
	type fields struct {
		Args []string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{name: "simple first", fields: fields{
			Args: []string{"a", "b"},
		}, want: "a"},
		{name: "null first", fields: fields{
			Args: nil,
		}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &Option{
				Args: tt.fields.Args,
			}
			assert.Equal(t, tt.want, o.First())
		})
	}
}

func TestMakeArguments(t *testing.T) {
	args := MakeArguments()
	assert.NotNil(t, args)
	assert.Equal(t, "", args.Op)
	assert.Empty(t, args.Options)
	assert.Empty(t, args.Targets)
}

func TestArguments_CopyGlobal(t *testing.T) {
	type fields struct {
		Op      string
		Options map[string]*Option
		Targets []string
	}
	tests := []struct {
		name   string
		fields fields
		want   *Arguments
	}{
		{name: "simple", fields: fields{
			Op: "Q",
			Options: map[string]*Option{
				"a": {}, "arch": {
					Global: true,
					Args:   []string{"x86_x64"},
				}, "boo": {Global: true, Args: []string{"a", "b"}},
			},
			Targets: []string{"a", "b"},
		}, want: &Arguments{
			Op: "",
			Options: map[string]*Option{
				"arch": {
					Global: true,
					Args:   []string{"x86_x64"},
				}, "boo": {Global: true, Args: []string{"a", "b"}},
			},
			Targets: []string{},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmdArgs := &Arguments{
				Op:      tt.fields.Op,
				Options: tt.fields.Options,
				Targets: tt.fields.Targets,
			}
			got := cmdArgs.CopyGlobal()
			assert.NotEqualValues(t, tt.fields.Options, got.Options)
			assert.NotEqualValues(t, tt.fields.Targets, got.Targets)
			assert.NotEqual(t, tt.fields.Op, got.Op)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestArguments_Copy(t *testing.T) {
	type fields struct {
		Op      string
		Options map[string]*Option
		Targets []string
	}
	tests := []struct {
		name   string
		fields fields
		want   *Arguments
	}{
		{name: "simple", fields: fields{
			Op: "Q",
			Options: map[string]*Option{
				"a": {}, "arch": {
					Args: []string{"x86_x64"}, Global: true,
				}, "boo": {Args: []string{"a", "b"}, Global: true},
			},
			Targets: []string{"a", "b"},
		}, want: &Arguments{
			Op: "Q",
			Options: map[string]*Option{
				"a": {}, "arch": {
					Global: true,
					Args:   []string{"x86_x64"},
				}, "boo": {Args: []string{"a", "b"}, Global: true},
			},
			Targets: []string{"a", "b"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmdArgs := &Arguments{
				Op:      tt.fields.Op,
				Options: tt.fields.Options,
				Targets: tt.fields.Targets,
			}
			got := cmdArgs.Copy()
			assert.Equal(t, cmdArgs, got)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestArguments_DelArg(t *testing.T) {
	args := MakeArguments()
	args.addParam("arch", "arg")
	args.addParam("ask", "arg")
	args.DelArg("arch", "ask")
	assert.Empty(t, args.Options)
}

func TestArguments_FormatArgs(t *testing.T) {
	type fields struct {
		Op      string
		Options map[string]*Option
		Targets []string
	}
	tests := []struct {
		name     string
		fields   fields
		wantArgs []string
	}{
		{name: "simple", fields: fields{
			Op:      "S",
			Options: map[string]*Option{},
			Targets: []string{"yay", "yay-bin", "yay-git"},
		}, wantArgs: []string{"-S"}},
		{name: "only global", fields: fields{
			Op:      "Y",
			Options: map[string]*Option{"noconfirm": {Global: true, Args: []string{""}}},
			Targets: []string{"yay", "yay-bin", "yay-git"},
		}, wantArgs: []string{"-Y"}},
		{name: "options single", fields: fields{
			Op:      "Y",
			Options: map[string]*Option{"overwrite": {Args: []string{"/tmp/a"}}, "useask": {Args: []string{""}}},
			Targets: []string{},
		}, wantArgs: []string{"-Y", "--overwrite", "/tmp/a", "--useask"}},
		{name: "options doubles", fields: fields{
			Op:      "Y",
			Options: map[string]*Option{"overwrite": {Args: []string{"/tmp/a", "/tmp/b", "/tmp/c"}}, "needed": {Args: []string{""}}},
			Targets: []string{},
		}, wantArgs: []string{"-Y", "--overwrite", "/tmp/a", "--overwrite", "/tmp/b", "--overwrite", "/tmp/c", "--needed"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmdArgs := &Arguments{
				Op:      tt.fields.Op,
				Options: tt.fields.Options,
				Targets: tt.fields.Targets,
			}
			gotArgs := cmdArgs.FormatArgs()
			assert.ElementsMatch(t, gotArgs, tt.wantArgs)
		})
	}
}

func TestArguments_FormatGlobalArgs(t *testing.T) {
	type fields struct {
		Op      string
		Options map[string]*Option
		Targets []string
	}
	tests := []struct {
		name     string
		fields   fields
		wantArgs []string
	}{
		{name: "simple", fields: fields{
			Op:      "S",
			Options: map[string]*Option{"dbpath": {Global: true, Args: []string{"/tmp/a", "/tmp/b"}}},
			Targets: []string{"yay", "yay-bin", "yay-git"},
		}, wantArgs: []string{"--dbpath", "/tmp/a", "--dbpath", "/tmp/b"}},
		{name: "only global", fields: fields{
			Op:      "Y",
			Options: map[string]*Option{"noconfirm": {Global: true, Args: []string{""}}},
			Targets: []string{"yay", "yay-bin", "yay-git"},
		}, wantArgs: []string{"--noconfirm"}},
		{name: "options single", fields: fields{
			Op:      "Y",
			Options: map[string]*Option{"overwrite": {Args: []string{"/tmp/a"}}, "useask": {Args: []string{""}}},
			Targets: []string{},
		}, wantArgs: []string(nil)},
		{name: "options doubles", fields: fields{
			Op:      "Y",
			Options: map[string]*Option{"overwrite": {Args: []string{"/tmp/a", "/tmp/b", "/tmp/c"}}, "needed": {Args: []string{""}}},
			Targets: []string{},
		}, wantArgs: []string(nil)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmdArgs := &Arguments{
				Op:      tt.fields.Op,
				Options: tt.fields.Options,
				Targets: tt.fields.Targets,
			}
			gotArgs := cmdArgs.FormatGlobals()
			assert.ElementsMatch(t, tt.wantArgs, gotArgs)
		})
	}
}

func Test_isArg(t *testing.T) {
	got := isArg("zorg")
	assert.False(t, got)

	got = isArg("dbpath")
	assert.True(t, got)
}
