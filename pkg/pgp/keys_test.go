package pgp

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"testing"

	gosrc "github.com/Morganamilo/go-srcinfo"
	"github.com/bradleyjkemp/cupaloy"
	rpc "github.com/mikkeloscar/aur"

	"github.com/Jguer/yay/v10/pkg/dep"
)

const (
	// The default port used by the PGP key server.
	gpgServerPort = 11371
)

func init() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		regex := regexp.MustCompile(`search=0[xX]([a-fA-F0-9]+)`)
		matches := regex.FindStringSubmatch(r.RequestURI)
		data := ""
		if matches != nil {
			data = getPgpKey(matches[1])
		}
		w.Header().Set("Content-Type", "application/pgp-keys")
		_, err := w.Write([]byte(data))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	})
}

func newPkg(basename string) *rpc.Pkg {
	return &rpc.Pkg{Name: basename, PackageBase: basename}
}

func getPgpKey(key string) string {
	var buffer bytes.Buffer

	if contents, err := ioutil.ReadFile(path.Join("testdata", key)); err == nil {
		buffer.WriteString("-----BEGIN PGP PUBLIC KEY BLOCK-----\n")
		buffer.WriteString("Version: SKS 1.1.6\n")
		buffer.WriteString("Comment: Hostname: yay\n\n")
		buffer.Write(contents)
		buffer.WriteString("\n-----END PGP PUBLIC KEY BLOCK-----\n")
	}
	return buffer.String()
}

func startPgpKeyServer() *http.Server {
	srv := &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", gpgServerPort)}

	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()
	return srv
}

func TestImportKeys(t *testing.T) {
	keyringDir, err := ioutil.TempDir("/tmp", "yay-test-keyring")
	if err != nil {
		t.Fatalf("Unable to init test keyring %q: %v\n", keyringDir, err)
	}
	defer os.RemoveAll(keyringDir)

	server := startPgpKeyServer()
	defer func() {
		err := server.Shutdown(context.TODO())
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()

	casetests := []struct {
		keys      []string
		wantError bool
	}{
		// Single key, should succeed.
		// C52048C0C0748FEE227D47A2702353E0F7E48EDB: Thomas Dickey.
		{
			keys:      []string{"C52048C0C0748FEE227D47A2702353E0F7E48EDB"},
			wantError: false,
		},
		// Two keys, should succeed as well.
		// 11E521D646982372EB577A1F8F0871F202119294: Tom Stellard.
		// B6C8F98282B944E3B0D5C2530FC3042E345AD05D: Hans Wennborg.
		{
			keys: []string{
				"11E521D646982372EB577A1F8F0871F202119294",
				"B6C8F98282B944E3B0D5C2530FC3042E345AD05D",
			},
			wantError: false,
		},
		// Single invalid key, should fail.
		{
			keys:      []string{"THIS-SHOULD-FAIL"},
			wantError: true,
		},
		// Two invalid keys, should fail.
		{
			keys:      []string{"THIS-SHOULD-FAIL", "THIS-ONE-SHOULD-FAIL-TOO"},
			wantError: true,
		},
		// Invalid + valid key. Should fail as well.
		// 647F28654894E3BD457199BE38DBBDC86092693E: Greg Kroah-Hartman.
		{
			keys: []string{
				"THIS-SHOULD-FAIL",
				"647F28654894E3BD457199BE38DBBDC86092693E",
			},
			wantError: true,
		},
	}

	for _, tt := range casetests {
		err := importKeys(tt.keys, "gpg", fmt.Sprintf("--homedir %s --keyserver 127.0.0.1", keyringDir))
		if !tt.wantError {
			if err != nil {
				t.Fatalf("Got error %q, want no error", err)
			}
			continue
		}
		// Here, we want to see the error.
		if err == nil {
			t.Fatalf("Got no error; want error")
		}
	}
}

func makeSrcinfo(pkgbase string, pgpkeys ...string) *gosrc.Srcinfo {
	srcinfo := gosrc.Srcinfo{}
	srcinfo.Pkgbase = pkgbase
	srcinfo.ValidPGPKeys = pgpkeys

	return &srcinfo
}

func TestCheckPgpKeys(t *testing.T) {
	keyringDir, err := ioutil.TempDir("/tmp", "yay-test-keyring")
	if err != nil {
		t.Fatalf("Unable to init test keyring: %v\n", err)
	}
	defer os.RemoveAll(keyringDir)

	server := startPgpKeyServer()
	defer func() {
		err := server.Shutdown(context.TODO())
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()

	casetests := []struct {
		name      string
		pkgs      dep.Base
		srcinfos  map[string]*gosrc.Srcinfo
		wantError bool
	}{
		// cower: single package, one valid key not yet in the keyring.
		// 487EACC08557AD082088DABA1EB2638FF56C0C53: Dave Reisner.
		{
			name:      " one valid key not yet in the keyring",
			pkgs:      dep.Base{newPkg("cower")},
			srcinfos:  map[string]*gosrc.Srcinfo{"cower": makeSrcinfo("cower", "487EACC08557AD082088DABA1EB2638FF56C0C53")},
			wantError: false,
		},
		// libc++: single package, two valid keys not yet in the keyring.
		// 11E521D646982372EB577A1F8F0871F202119294: Tom Stellard.
		// B6C8F98282B944E3B0D5C2530FC3042E345AD05D: Hans Wennborg.
		{
			name: "two valid keys not yet in the keyring",
			pkgs: dep.Base{newPkg("libc++")},
			srcinfos: map[string]*gosrc.Srcinfo{
				"libc++": makeSrcinfo("libc++", "11E521D646982372EB577A1F8F0871F202119294", "B6C8F98282B944E3B0D5C2530FC3042E345AD05D"),
			},
			wantError: false,
		},
		// Two dummy packages requiring the same key.
		// ABAF11C65A2970B130ABE3C479BE3E4300411886: Linus Torvalds.
		{
			name: "Two dummy packages requiring the same key",
			pkgs: dep.Base{newPkg("dummy-1"), newPkg("dummy-2")},
			srcinfos: map[string]*gosrc.Srcinfo{
				"dummy-1": makeSrcinfo("dummy-1",
					"ABAF11C65A2970B130ABE3C479BE3E4300411886"),
				"dummy-2": makeSrcinfo("dummy-2", "ABAF11C65A2970B130ABE3C479BE3E4300411886"),
			},
			wantError: false,
		},
		// dummy package: single package, two valid keys, one of them already
		// in the keyring.
		// 11E521D646982372EB577A1F8F0871F202119294: Tom Stellard.
		// C52048C0C0748FEE227D47A2702353E0F7E48EDB: Thomas Dickey.
		{
			name: "one already in keyring",
			pkgs: dep.Base{newPkg("dummy-3")},
			srcinfos: map[string]*gosrc.Srcinfo{
				"dummy-3": makeSrcinfo("dummy-3", "11E521D646982372EB577A1F8F0871F202119294", "C52048C0C0748FEE227D47A2702353E0F7E48EDB"),
			},
			wantError: false,
		},
		// Two dummy packages with existing keys.
		{
			name: "two existing",
			pkgs: dep.Base{newPkg("dummy-4"), newPkg("dummy-5")},
			srcinfos: map[string]*gosrc.Srcinfo{
				"dummy-4": makeSrcinfo("dummy-4", "11E521D646982372EB577A1F8F0871F202119294"),
				"dummy-5": makeSrcinfo("dummy-5", "C52048C0C0748FEE227D47A2702353E0F7E48EDB"),
			},
			wantError: false,
		},
		// Dummy package with invalid key, should fail.
		{
			name:      "one invalid",
			pkgs:      dep.Base{newPkg("dummy-7")},
			srcinfos:  map[string]*gosrc.Srcinfo{"dummy-7": makeSrcinfo("dummy-7", "THIS-SHOULD-FAIL")},
			wantError: true,
		},
		// Dummy package with both an invalid an another valid key, should fail.
		// A314827C4E4250A204CE6E13284FC34C8E4B1A25: Thomas Bächler.
		{
			name:      "one invalid, one valid",
			pkgs:      dep.Base{newPkg("dummy-8")},
			srcinfos:  map[string]*gosrc.Srcinfo{"dummy-8": makeSrcinfo("dummy-8", "A314827C4E4250A204CE6E13284FC34C8E4B1A25", "THIS-SHOULD-FAIL")},
			wantError: true,
		},
	}

	for _, tt := range casetests {
		t.Run(tt.name, func(t *testing.T) {
			rescueStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			err := CheckPgpKeys([]dep.Base{tt.pkgs}, tt.srcinfos, "gpg",
				fmt.Sprintf("--homedir %s --keyserver 127.0.0.1", keyringDir), true)
			if !tt.wantError {
				if err != nil {
					t.Fatalf("Got error %q, want no error", err)
				}

				w.Close()
				out, _ := ioutil.ReadAll(r)
				os.Stdout = rescueStdout

				cupaloy.SnapshotT(t, string(out))
				return
			}
			// Here, we want to see the error.
			if err == nil {
				t.Fatalf("Got no error; want error")
			}
		})
	}
}
