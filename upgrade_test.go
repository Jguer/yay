package main

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/Jguer/yay/v10/pkg/db"
	"github.com/Jguer/yay/v10/pkg/db/mock"
	"github.com/Jguer/yay/v10/pkg/upgrade"
	"github.com/bradleyjkemp/cupaloy"
	rpc "github.com/mikkeloscar/aur"
	"github.com/stretchr/testify/assert"
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

func Test_upDevel(t *testing.T) {
	type args struct {
		remote  []db.RepoPackage
		aurdata map[string]*rpc.Pkg
		cached  vcsInfo
	}
	tests := []struct {
		name string
		args args
		want upgrade.UpSlice
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
			args: args{
				cached: vcsInfo{
					"hello": shaInfos{
						"github.com/Jguer/yay.git": shaInfo{
							Protocols: []string{"https"},
							Branch:    "main",
							SHA:       "991c5b4146fd27f4aacf4e3111258a848934aaa1"}}},
				remote:  []db.RepoPackage{&mock.Package{PName: "hello", PVersion: "2.0.0"}},
				aurdata: map[string]*rpc.Pkg{"hello": {Version: "2.1.0", Name: "hello"}},
			},
			want: upgrade.UpSlice{upgrade.Upgrade{Name: "hello", Repository: "aur", LocalVersion: "2.0.0", RemoteVersion: "2.1.0"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := upDevel(tt.args.remote, tt.args.aurdata, tt.args.cached)
			assert.EqualValues(t, tt.want, got)
		})
	}
}
