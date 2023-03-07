package pgp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"

	gosrc "github.com/Morganamilo/go-srcinfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v12/pkg/settings/exe"
)

func makeSrcinfo(pkgbase string, pgpkeys ...string) *gosrc.Srcinfo {
	srcinfo := gosrc.Srcinfo{}
	srcinfo.Pkgbase = pkgbase
	srcinfo.ValidPGPKeys = pgpkeys

	return &srcinfo
}

func TestCheckPgpKeys(t *testing.T) {
	gpgBin := t.TempDir() + "/gpg"

	f, err := os.OpenFile(gpgBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	testcases := []struct {
		name        string
		pkgs        map[string]string
		srcinfos    map[string]*gosrc.Srcinfo
		wantError   bool
		wantShow    []string
		wantCapture []string
		showFn      func(cmd *exec.Cmd) error
		expected    []string
	}{
		// cower: single package, one valid key not yet in the keyring.
		// 487EACC08557AD082088DABA1EB2638FF56C0C53: Dave Reisner.
		{
			name:      " one valid key not yet in the keyring",
			pkgs:      map[string]string{"cower": ""},
			srcinfos:  map[string]*gosrc.Srcinfo{"cower": makeSrcinfo("cower", "487EACC08557AD082088DABA1EB2638FF56C0C53")},
			wantError: false,
			wantShow: []string{
				"gpg --homedir /tmp --list-keys 487EACC08557AD082088DABA1EB2638FF56C0C53",
				"gpg --homedir /tmp --recv-keys 487EACC08557AD082088DABA1EB2638FF56C0C53",
			},
			wantCapture: []string{},
			showFn: func(cmd *exec.Cmd) error {
				s := cmd.String()
				if strings.Contains(s, "--list-keys") {
					return fmt.Errorf("key not found")
				}
				return nil
			},
			expected: []string{"487EACC08557AD082088DABA1EB2638FF56C0C53"},
		},
		// libc++: single package, two valid keys not yet in the keyring.
		// 11E521D646982372EB577A1F8F0871F202119294: Tom Stellard.
		// B6C8F98282B944E3B0D5C2530FC3042E345AD05D: Hans Wennborg.
		{
			name:      "two valid keys not yet in the keyring",
			pkgs:      map[string]string{"libc++": ""},
			srcinfos:  map[string]*gosrc.Srcinfo{"libc++": makeSrcinfo("libc++", "11E521D646982372EB577A1F8F0871F202119294", "B6C8F98282B944E3B0D5C2530FC3042E345AD05D")},
			wantError: false,
			wantShow: []string{
				"gpg --homedir /tmp --list-keys 11E521D646982372EB577A1F8F0871F202119294",
				"gpg --homedir /tmp --list-keys B6C8F98282B944E3B0D5C2530FC3042E345AD05D",
				"gpg --homedir /tmp --recv-keys 11E521D646982372EB577A1F8F0871F202119294 B6C8F98282B944E3B0D5C2530FC3042E345AD05D",
			},
			wantCapture: []string{},
			showFn: func(cmd *exec.Cmd) error {
				s := cmd.String()
				if strings.Contains(s, "--list-keys") {
					return fmt.Errorf("key not found")
				}

				return nil
			},
			expected: []string{"11E521D646982372EB577A1F8F0871F202119294", "B6C8F98282B944E3B0D5C2530FC3042E345AD05D"},
		},
		{
			name: "Two dummy packages requiring the same key",
			pkgs: map[string]string{"dummy-1": "", "dummy-2": ""},
			srcinfos: map[string]*gosrc.Srcinfo{
				"dummy-1": makeSrcinfo("dummy-1",
					"ABAF11C65A2970B130ABE3C479BE3E4300411886"),
				"dummy-2": makeSrcinfo("dummy-2", "ABAF11C65A2970B130ABE3C479BE3E4300411886"),
			},
			wantError:   false,
			expected:    []string{"ABAF11C65A2970B130ABE3C479BE3E4300411886"},
			wantCapture: []string{},
			wantShow: []string{
				"gpg --homedir /tmp --list-keys ABAF11C65A2970B130ABE3C479BE3E4300411886",
				"gpg --homedir /tmp --recv-keys ABAF11C65A2970B130ABE3C479BE3E4300411886",
			},
			showFn: func(cmd *exec.Cmd) error {
				s := cmd.String()
				if strings.Contains(s, "--list-keys") {
					return fmt.Errorf("key not found")
				}

				return nil
			},
		},
		// dummy package: single package, two valid keys, one of them already
		// in the keyring.
		// 11E521D646982372EB577A1F8F0871F202119294: Tom Stellard.
		// C52048C0C0748FEE227D47A2702353E0F7E48EDB: Thomas Dickey.
		{
			name: "one already in keyring",
			pkgs: map[string]string{"dummy-3": ""},
			srcinfos: map[string]*gosrc.Srcinfo{
				"dummy-3": makeSrcinfo("dummy-3", "11E521D646982372EB577A1F8F0871F202119294", "C52048C0C0748FEE227D47A2702353E0F7E48EDB"),
			},
			wantError:   false,
			expected:    []string{"C52048C0C0748FEE227D47A2702353E0F7E48EDB"},
			wantCapture: []string{},
			showFn: func(cmd *exec.Cmd) error {
				s := cmd.String()
				if strings.Contains(s, "--list-keys") &&
					!strings.Contains(s, "11E521D646982372EB577A1F8F0871F202119294") {
					return fmt.Errorf("key not found")
				}

				return nil
			},
			wantShow: []string{
				"gpg --homedir /tmp --list-keys 11E521D646982372EB577A1F8F0871F202119294",
				"gpg --homedir /tmp --list-keys C52048C0C0748FEE227D47A2702353E0F7E48EDB",
				"gpg --homedir /tmp --recv-keys C52048C0C0748FEE227D47A2702353E0F7E48EDB",
			},
		},
		// Two dummy packages with existing keys.
		{
			name: "two existing",
			pkgs: map[string]string{"dummy-4": "", "dummy-5": ""},
			srcinfos: map[string]*gosrc.Srcinfo{
				"dummy-4": makeSrcinfo("dummy-4", "11E521D646982372EB577A1F8F0871F202119294"),
				"dummy-5": makeSrcinfo("dummy-5", "C52048C0C0748FEE227D47A2702353E0F7E48EDB"),
			},
			wantError:   false,
			expected:    []string{},
			wantCapture: []string{},
			showFn: func(cmd *exec.Cmd) error {
				return nil
			},
			wantShow: []string{
				"gpg --homedir /tmp --list-keys 11E521D646982372EB577A1F8F0871F202119294",
				"gpg --homedir /tmp --list-keys C52048C0C0748FEE227D47A2702353E0F7E48EDB",
			},
		},
		// Dummy package with invalid key, should fail.
		{
			name:        "one invalid",
			pkgs:        map[string]string{"dummy-7": ""},
			srcinfos:    map[string]*gosrc.Srcinfo{"dummy-7": makeSrcinfo("dummy-7", "THIS-SHOULD-FAIL")},
			wantError:   true,
			wantCapture: []string{},
			wantShow: []string{
				"gpg --homedir /tmp --list-keys THIS-SHOULD-FAIL",
				"gpg --homedir /tmp --recv-keys THIS-SHOULD-FAIL",
			},
			showFn: func(cmd *exec.Cmd) error {
				s := cmd.String()
				if strings.Contains(s, "--list-keys") {
					return fmt.Errorf("key not found")
				}

				if strings.Contains(s, "--recv-keys") {
					return fmt.Errorf("invalid key")
				}

				return nil
			},
		},
		// Dummy package with both an invalid an another valid key, should fail.
		// A314827C4E4250A204CE6E13284FC34C8E4B1A25: Thomas Bächler.
		{
			name:        "one invalid, one valid",
			pkgs:        map[string]string{"dummy-8": ""},
			srcinfos:    map[string]*gosrc.Srcinfo{"dummy-8": makeSrcinfo("dummy-8", "A314827C4E4250A204CE6E13284FC34C8E4B1A25", "THIS-SHOULD-FAIL")},
			wantError:   true,
			expected:    []string{},
			wantCapture: []string{},
			showFn: func(cmd *exec.Cmd) error {
				s := cmd.String()
				if strings.Contains(s, "--list-keys") {
					return fmt.Errorf("key not found")
				}

				if strings.Contains(s, "--recv-keys") {
					return fmt.Errorf("invalid key")
				}

				return nil
			},
			wantShow: []string{
				"gpg --homedir /tmp --list-keys A314827C4E4250A204CE6E13284FC34C8E4B1A25",
				"gpg --homedir /tmp --list-keys THIS-SHOULD-FAIL",
				"gpg --homedir /tmp --recv-keys A314827C4E4250A204CE6E13284FC34C8E4B1A25 THIS-SHOULD-FAIL",
			},
		},
	}

	for _, tt := range testcases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := &exe.MockRunner{
				ShowFn: tt.showFn,
				CaptureFn: func(cmd *exec.Cmd) (stdout string, stderr string, err error) {
					return "", "", nil
				},
			}

			cmdBuilder := exe.CmdBuilder{
				GPGBin:   gpgBin,
				GPGFlags: []string{"--homedir /tmp"},
				Runner:   mockRunner,
			}
			problematic, err := CheckPgpKeys(context.Background(), tt.pkgs, tt.srcinfos, &cmdBuilder, true)

			require.Len(t, mockRunner.ShowCalls, len(tt.wantShow))
			require.Len(t, mockRunner.CaptureCalls, len(tt.wantCapture))

			sort.SliceStable(mockRunner.ShowCalls, func(i, j int) bool {
				return mockRunner.ShowCalls[i].Args[0].(*exec.Cmd).String() < mockRunner.ShowCalls[j].Args[0].(*exec.Cmd).String()
			})
			for i, call := range mockRunner.ShowCalls {
				show := call.Args[0].(*exec.Cmd).String()
				show = strings.ReplaceAll(show, gpgBin, "gpg")

				// options are in a different order on different systems and on CI root user is used
				assert.Subset(t, strings.Split(show, " "), strings.Split(tt.wantShow[i], " "), show)
			}

			for i, call := range mockRunner.CaptureCalls {
				capture := call.Args[0].(*exec.Cmd).String()
				capture = strings.ReplaceAll(capture, gpgBin, "gpg")
				assert.Subset(t, strings.Split(capture, " "), strings.Split(tt.wantCapture[i], " "), capture)
			}

			if tt.wantError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			assert.ElementsMatch(t, tt.expected, problematic, fmt.Sprintf("%#v", problematic))
		})
	}
}
