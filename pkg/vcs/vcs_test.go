package vcs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"testing"

	gosrc "github.com/Morganamilo/go-srcinfo"
	"github.com/bradleyjkemp/cupaloy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v11/pkg/settings/exe"
)

func TestParsing(t *testing.T) {
	t.Parallel()
	type source struct {
		URL       string
		Branch    string
		Protocols []string
	}

	urls := []string{
		"git+https://github.com/neovim/neovim.git",
		"git://github.com/jguer/yay.git#branch=master",
		"git://github.com/davidgiven/ack",
		"git://github.com/jguer/yay.git#tag=v3.440",
		"git://github.com/jguer/yay.git#commit=e5470c88c6e2f9e0f97deb4728659ffa70ef5d0c",
		"a+b+c+d+e+f://github.com/jguer/yay.git#branch=foo",
	}

	sources := []source{
		{"github.com/neovim/neovim.git", "HEAD", []string{"https"}},
		{"github.com/jguer/yay.git", "master", []string{"git"}},
		{"github.com/davidgiven/ack", "HEAD", []string{"git"}},
		{"", "", nil},
		{"", "", nil},
		{"", "", nil},
	}

	for n, url := range urls {
		url, branch, protocols := parseSource(url)
		compare := sources[n]

		assert.Equal(t, compare.URL, url)
		assert.Equal(t, compare.Branch, branch)
		assert.Equal(t, compare.Protocols, protocols)
	}
}

func TestNewInfoStore(t *testing.T) {
	t.Parallel()
	type args struct {
		filePath   string
		cmdBuilder *exe.CmdBuilder
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "normal",
			args: args{
				"/tmp/a.json",
				&exe.CmdBuilder{GitBin: "git", GitFlags: []string{"--a", "--b"}, Runner: &exe.OSRunner{}},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := NewInfoStore(tt.args.filePath, tt.args.cmdBuilder)
			assert.NotNil(t, got)
			assert.Equal(t, []string{"--a", "--b"}, got.CmdBuilder.(*exe.CmdBuilder).GitFlags)
			assert.Equal(t, tt.args.cmdBuilder, got.CmdBuilder)
			assert.Equal(t, "/tmp/a.json", got.FilePath)
		})
	}
}

type MockRunner struct {
	Returned []string
	Index    int
}

func (r *MockRunner) Show(cmd *exec.Cmd) error {
	return nil
}

func (r *MockRunner) Capture(cmd *exec.Cmd) (stdout, stderr string, err error) {
	stdout = r.Returned[r.Index]
	if r.Returned[0] == "error" {
		err = errors.New("possible error")
	}
	return stdout, stderr, err
}

func TestInfoStore_NeedsUpdate(t *testing.T) {
	t.Parallel()
	type fields struct {
		CmdBuilder *exe.CmdBuilder
	}
	type args struct {
		infos OriginInfoByURL
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "simple-has_update",
			args: args{infos: OriginInfoByURL{
				"github.com/Jguer/z.git": OriginInfo{
					Protocols: []string{"https"},
					Branch:    "0",
					SHA:       "991c5b4146fd27f4aacf4e3111258a848934aaa1",
				},
			}}, fields: fields{
				CmdBuilder: &exe.CmdBuilder{GitBin: "git", GitFlags: []string{""}, Runner: &MockRunner{
					Returned: []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa	HEAD"},
				}},
			},
			want: true,
		},
		{
			name: "double-has_update",
			args: args{infos: OriginInfoByURL{
				"github.com/Jguer/z.git": OriginInfo{
					Protocols: []string{"https"},
					Branch:    "0",
					SHA:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				},
				"github.com/Jguer/a.git": OriginInfo{
					Protocols: []string{"https"},
					Branch:    "0",
					SHA:       "991c5b4146fd27f4aacf4e3111258a848934aaa1",
				},
			}}, fields: fields{
				CmdBuilder: &exe.CmdBuilder{GitBin: "git", GitFlags: []string{""}, Runner: &MockRunner{
					Returned: []string{
						"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa	HEAD",
						"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa	HEAD",
					},
				}},
			},
			want: true,
		},
		{
			name: "simple-no_update",
			args: args{infos: OriginInfoByURL{
				"github.com/Jguer/z.git": OriginInfo{
					Protocols: []string{"https"},
					Branch:    "0",
					SHA:       "991c5b4146fd27f4aacf4e3111258a848934aaa1",
				},
			}}, fields: fields{
				CmdBuilder: &exe.CmdBuilder{GitBin: "git", GitFlags: []string{""}, Runner: &MockRunner{
					Returned: []string{"991c5b4146fd27f4aacf4e3111258a848934aaa1	HEAD"},
				}},
			},
			want: false,
		},
		{
			name: "simple-no_split",
			args: args{infos: OriginInfoByURL{
				"github.com/Jguer/z.git": OriginInfo{
					Protocols: []string{"https"},
					Branch:    "0",
					SHA:       "991c5b4146fd27f4aacf4e3111258a848934aaa1",
				},
			}}, fields: fields{
				CmdBuilder: &exe.CmdBuilder{GitBin: "git", GitFlags: []string{""}, Runner: &MockRunner{
					Returned: []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
				}},
			},
			want: false,
		},
		{
			name: "simple-error",
			args: args{infos: OriginInfoByURL{
				"github.com/Jguer/z.git": OriginInfo{
					Protocols: []string{"https"},
					Branch:    "0",
					SHA:       "991c5b4146fd27f4aacf4e3111258a848934aaa1",
				},
			}}, fields: fields{
				CmdBuilder: &exe.CmdBuilder{
					GitBin: "git", GitFlags: []string{""},
					Runner: &MockRunner{
						Returned: []string{"error"},
					},
				},
			},
			want: false,
		},
		{
			name: "simple-no protocol",
			args: args{infos: OriginInfoByURL{
				"github.com/Jguer/z.git": OriginInfo{
					Protocols: []string{},
					Branch:    "0",
					SHA:       "991c5b4146fd27f4aacf4e3111258a848934aaa1",
				},
			}}, fields: fields{
				CmdBuilder: &exe.CmdBuilder{GitBin: "git", GitFlags: []string{""}, Runner: &MockRunner{
					Returned: []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
				}},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := &InfoStore{
				CmdBuilder: tt.fields.CmdBuilder,
			}
			got := v.NeedsUpdate(context.TODO(), tt.args.infos)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestInfoStore_Update(t *testing.T) {
	t.Parallel()
	type fields struct {
		OriginsByPackage map[string]OriginInfoByURL
		CmdBuilder       *exe.CmdBuilder
	}
	type args struct {
		pkgName string
		sources []gosrc.ArchString
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "simple",
			args: args{
				pkgName: "hello",
				sources: []gosrc.ArchString{{Value: "git://github.com/jguer/yay.git#branch=master"}},
			},
			fields: fields{
				OriginsByPackage: make(map[string]OriginInfoByURL),
				CmdBuilder: &exe.CmdBuilder{
					GitBin: "git", GitFlags: []string{""},
					Runner: &MockRunner{Returned: []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa HEAD"}},
				},
			},
		},
	}

	file, err := os.CreateTemp("/tmp", "yay-infostore-*-test")
	filePath := file.Name()
	require.NoError(t, err)

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := &InfoStore{
				OriginsByPackage: tt.fields.OriginsByPackage,
				FilePath:         filePath,
				CmdBuilder:       tt.fields.CmdBuilder,
			}
			var mux sync.Mutex
			var wg sync.WaitGroup
			wg.Add(1)
			v.Update(context.TODO(), tt.args.pkgName, tt.args.sources, &mux, &wg)
			wg.Wait()
			assert.Len(t, tt.fields.OriginsByPackage, 1)

			marshalledinfo, err := json.MarshalIndent(tt.fields.OriginsByPackage, "", "\t")
			assert.NoError(t, err)

			cupaloy.SnapshotT(t, marshalledinfo)

			v.Load()
			fmt.Println(v.OriginsByPackage)
			assert.Len(t, tt.fields.OriginsByPackage, 1)

			marshalledinfo, err = json.MarshalIndent(tt.fields.OriginsByPackage, "", "\t")
			assert.NoError(t, err)

			cupaloy.SnapshotT(t, marshalledinfo)
		})
	}

	require.NoError(t, os.Remove(filePath))
}

func TestInfoStore_Remove(t *testing.T) {
	t.Parallel()
	type fields struct {
		OriginsByPackage map[string]OriginInfoByURL
	}
	type args struct {
		pkgs []string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "simple",
			args: args{pkgs: []string{"a", "c"}},
			fields: fields{
				OriginsByPackage: map[string]OriginInfoByURL{
					"a": {},
					"b": {},
					"c": {},
					"d": {},
				},
			},
		},
	}

	file, err := os.CreateTemp("/tmp", "yay-vcs-*-test")
	filePath := file.Name()
	require.NoError(t, err)

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := &InfoStore{
				OriginsByPackage: tt.fields.OriginsByPackage,
				FilePath:         filePath,
			}
			v.RemovePackage(tt.args.pkgs)
			assert.Len(t, tt.fields.OriginsByPackage, 2)
		})
	}

	require.NoError(t, os.Remove(filePath))
}
