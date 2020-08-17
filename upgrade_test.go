package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/bradleyjkemp/cupaloy"
	rpc "github.com/mikkeloscar/aur"
	"github.com/stretchr/testify/assert"

	"github.com/Jguer/yay/v10/pkg/db"
	"github.com/Jguer/yay/v10/pkg/db/mock"
	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/upgrade"
)

func Test_upAUR(t *testing.T) {
	type args struct {
		remote     []db.RepoPackage
		aurdata    map[string]*rpc.Pkg
		timeUpdate bool
	}
	tests := []struct {
		name string
		args args
		want upgrade.UpSlice
	}{
		{name: "No Updates",
			args: args{
				remote: []db.RepoPackage{
					&mock.Package{PName: "hello", PVersion: "2.0.0"},
					&mock.Package{PName: "local_pkg", PVersion: "1.1.0"},
					&mock.Package{PName: "ignored", PVersion: "1.0.0", PShouldIgnore: true}},
				aurdata: map[string]*rpc.Pkg{
					"hello":   {Version: "2.0.0", Name: "hello"},
					"ignored": {Version: "2.0.0", Name: "ignored"}},
				timeUpdate: false,
			},
			want: upgrade.UpSlice{}},
		{name: "Simple Update",
			args: args{
				remote:     []db.RepoPackage{&mock.Package{PName: "hello", PVersion: "2.0.0"}},
				aurdata:    map[string]*rpc.Pkg{"hello": {Version: "2.1.0", Name: "hello"}},
				timeUpdate: false,
			},
			want: upgrade.UpSlice{upgrade.Upgrade{Name: "hello", Repository: "aur", LocalVersion: "2.0.0", RemoteVersion: "2.1.0"}}},
		{name: "Time Update",
			args: args{
				remote:     []db.RepoPackage{&mock.Package{PName: "hello", PVersion: "2.0.0", PBuildDate: time.Now()}},
				aurdata:    map[string]*rpc.Pkg{"hello": {Version: "2.0.0", Name: "hello", LastModified: int(time.Now().AddDate(0, 0, 2).Unix())}},
				timeUpdate: true,
			},
			want: upgrade.UpSlice{upgrade.Upgrade{Name: "hello", Repository: "aur", LocalVersion: "2.0.0", RemoteVersion: "2.0.0"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rescueStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w
			got := upAUR(tt.args.remote, tt.args.aurdata, tt.args.timeUpdate)
			assert.EqualValues(t, tt.want, got)

			w.Close()
			out, _ := ioutil.ReadAll(r)
			cupaloy.SnapshotT(t, out)
			os.Stdout = rescueStdout
		})
	}
}

type MockRunner struct {
	Returned []string
	Index    int
	t        *testing.T
}

func (r *MockRunner) Show(cmd *exec.Cmd) error {
	return nil
}

func (r *MockRunner) Capture(cmd *exec.Cmd, timeout int64) (stdout, stderr string, err error) {
	i, _ := strconv.Atoi(cmd.Args[len(cmd.Args)-1])
	if i >= len(r.Returned) {
		fmt.Println(r.Returned)
		fmt.Println(cmd.Args)
		fmt.Println(i)
	}
	stdout = r.Returned[i]
	assert.Contains(r.t, cmd.Args, "ls-remote")
	return stdout, stderr, err
}

func Test_upDevel(t *testing.T) {
	config = settings.MakeConfig()
	config.Runtime, _ = settings.MakeRuntime()
	config.Runtime.CmdRunner = &MockRunner{
		Returned: []string{
			"7f4c277ce7149665d1c79b76ca8fbb832a65a03b	HEAD",
			"7f4c277ce7149665d1c79b76ca8fbb832a65a03b	HEAD",
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa	HEAD",
			"cccccccccccccccccccccccccccccccccccccccc	HEAD",
			"991c5b4146fd27f4aacf4e3111258a848934aaa1	HEAD",
		},
	}

	type args struct {
		remote  []db.RepoPackage
		aurdata map[string]*rpc.Pkg
		cached  vcsInfo
	}
	tests := []struct {
		name     string
		args     args
		want     upgrade.UpSlice
		finalLen int
	}{
		{name: "No Updates",
			args: args{
				cached: vcsInfo{},
				remote: []db.RepoPackage{
					&mock.Package{PName: "hello", PVersion: "2.0.0"},
					&mock.Package{PName: "local_pkg", PVersion: "1.1.0"},
					&mock.Package{PName: "ignored", PVersion: "1.0.0", PShouldIgnore: true}},
				aurdata: map[string]*rpc.Pkg{
					"hello":   {Version: "2.0.0", Name: "hello"},
					"ignored": {Version: "2.0.0", Name: "ignored"}},
			},
			want: upgrade.UpSlice{}},
		{name: "Simple Update",
			finalLen: 3,
			args: args{
				cached: vcsInfo{
					"hello": shaInfos{
						"github.com/Jguer/z.git": shaInfo{
							Protocols: []string{"https"},
							Branch:    "0",
							SHA:       "991c5b4146fd27f4aacf4e3111258a848934aaa1"}},
					"hello-non-existant": shaInfos{
						"github.com/Jguer/y.git": shaInfo{
							Protocols: []string{"https"},
							Branch:    "0",
							SHA:       "991c5b4146fd27f4aacf4e3111258a848934aaa1"}},
					"hello2": shaInfos{
						"github.com/Jguer/a.git": shaInfo{
							Protocols: []string{"https"},
							Branch:    "1",
							SHA:       "7f4c277ce7149665d1c79b76ca8fbb832a65a03b"}},
					"hello4": shaInfos{
						"github.com/Jguer/b.git": shaInfo{
							Protocols: []string{"https"},
							Branch:    "2",
							SHA:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
						"github.com/Jguer/c.git": shaInfo{
							Protocols: []string{"https"},
							Branch:    "3",
							SHA:       "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
					},
				},
				remote: []db.RepoPackage{
					&mock.Package{PName: "hello", PVersion: "2.0.0"},
					&mock.Package{PName: "hello2", PVersion: "3.0.0"},
					&mock.Package{PName: "hello4", PVersion: "4.0.0"}},
				aurdata: map[string]*rpc.Pkg{
					"hello":  {Version: "2.0.0", Name: "hello"},
					"hello2": {Version: "2.0.0", Name: "hello2"},
					"hello4": {Version: "2.0.0", Name: "hello4"},
				},
			},
			want: upgrade.UpSlice{upgrade.Upgrade{
				Name:          "hello",
				Repository:    "devel",
				LocalVersion:  "2.0.0",
				RemoteVersion: "latest-commit"},
				upgrade.Upgrade{
					Name:          "hello4",
					Repository:    "devel",
					LocalVersion:  "4.0.0",
					RemoteVersion: "latest-commit"},
			}},
		{name: "No update returned",
			finalLen: 1,
			args: args{
				cached: vcsInfo{
					"hello": shaInfos{
						"github.com/Jguer/d.git": shaInfo{
							Protocols: []string{"https"},
							Branch:    "4",
							SHA:       "991c5b4146fd27f4aacf4e3111258a848934aaa1"}}},
				remote:  []db.RepoPackage{&mock.Package{PName: "hello", PVersion: "2.0.0"}},
				aurdata: map[string]*rpc.Pkg{"hello": {Version: "2.0.0", Name: "hello"}},
			},
			want: upgrade.UpSlice{}},
		{name: "No update returned - ignored",
			finalLen: 1,
			args: args{
				cached: vcsInfo{
					"hello": shaInfos{
						"github.com/Jguer/e.git": shaInfo{
							Protocols: []string{"https"},
							Branch:    "3",
							SHA:       "991c5b4146fd27f4aacf4e3111258a848934aaa1"}}},
				remote:  []db.RepoPackage{&mock.Package{PName: "hello", PVersion: "2.0.0", PShouldIgnore: true}},
				aurdata: map[string]*rpc.Pkg{"hello": {Version: "2.0.0", Name: "hello"}},
			},
			want: upgrade.UpSlice{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Runtime.CmdRunner.(*MockRunner).t = t
			got := upDevel(tt.args.remote, tt.args.aurdata, tt.args.cached)
			assert.ElementsMatch(t, tt.want, got)
			assert.Equal(t, tt.finalLen, len(tt.args.cached))
		})
	}
}
