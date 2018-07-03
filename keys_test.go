package main

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
	rpc "github.com/mikkeloscar/aur"
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
		w.Write([]byte(data))
	})
}

func newPkg(basename string) *rpc.Pkg {
	return &rpc.Pkg{Name: basename, PackageBase: basename}
}

func getPgpKey(key string) string {
	var buffer bytes.Buffer

	if contents, err := ioutil.ReadFile(path.Join("testdata", "keys", key)); err == nil {
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
		srv.ListenAndServe()
	}()
	return srv
}

func TestImportKeys(t *testing.T) {
	keyringDir, err := ioutil.TempDir("/tmp", "yay-test-keyring")
	if err != nil {
		t.Fatalf("Unable to init test keyring %q: %v\n", keyringDir, err)
	}
	defer os.RemoveAll(keyringDir)

	config.GpgBin = "gpg"
	config.GpgFlags = fmt.Sprintf("--homedir %s --keyserver 127.0.0.1", keyringDir)

	server := startPgpKeyServer()
	defer server.Shutdown(context.TODO())

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
			keys: []string{"11E521D646982372EB577A1F8F0871F202119294",
				"B6C8F98282B944E3B0D5C2530FC3042E345AD05D"},
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
			keys: []string{"THIS-SHOULD-FAIL",
				"647F28654894E3BD457199BE38DBBDC86092693E"},
			wantError: true,
		},
	}

	for _, tt := range casetests {
		err := importKeys(tt.keys)
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

	config.GpgBin = "gpg"
	config.GpgFlags = fmt.Sprintf("--homedir %s --keyserver 127.0.0.1", keyringDir)

	server := startPgpKeyServer()
	defer server.Shutdown(context.TODO())

	casetests := []struct {
		pkgs      []*rpc.Pkg
		srcinfos  map[string]*gosrc.Srcinfo
		bases     map[string][]*rpc.Pkg
		wantError bool
	}{
		// cower: single package, one valid key not yet in the keyring.
		// 487EACC08557AD082088DABA1EB2638FF56C0C53: Dave Reisner.
		{
			pkgs:      []*rpc.Pkg{newPkg("cower")},
			srcinfos:  map[string]*gosrc.Srcinfo{"cower": makeSrcinfo("cower", "487EACC08557AD082088DABA1EB2638FF56C0C53")},
			bases:     map[string][]*rpc.Pkg{"cower": {newPkg("cower")}},
			wantError: false,
		},
		// libc++: single package, two valid keys not yet in the keyring.
		// 11E521D646982372EB577A1F8F0871F202119294: Tom Stellard.
		// B6C8F98282B944E3B0D5C2530FC3042E345AD05D: Hans Wennborg.
		{
			pkgs:      []*rpc.Pkg{newPkg("libc++")},
			srcinfos:  map[string]*gosrc.Srcinfo{"libc++": makeSrcinfo("libc++", "11E521D646982372EB577A1F8F0871F202119294", "B6C8F98282B944E3B0D5C2530FC3042E345AD05D")},
			bases:     map[string][]*rpc.Pkg{"libc++": {newPkg("libc++")}},
			wantError: false,
		},
		// Two dummy packages requiring the same key.
		// ABAF11C65A2970B130ABE3C479BE3E4300411886: Linus Torvalds.
		{
			pkgs:      []*rpc.Pkg{newPkg("dummy-1"), newPkg("dummy-2")},
			srcinfos:  map[string]*gosrc.Srcinfo{"dummy-1": makeSrcinfo("dummy-1", "ABAF11C65A2970B130ABE3C479BE3E4300411886"), "dummy-2": makeSrcinfo("dummy-2", "ABAF11C65A2970B130ABE3C479BE3E4300411886")},
			bases:     map[string][]*rpc.Pkg{"dummy-1": {newPkg("dummy-1")}, "dummy-2": {newPkg("dummy-2")}},
			wantError: false,
		},
		// dummy package: single package, two valid keys, one of them already
		// in the keyring.
		// 11E521D646982372EB577A1F8F0871F202119294: Tom Stellard.
		// C52048C0C0748FEE227D47A2702353E0F7E48EDB: Thomas Dickey.
		{
			pkgs:      []*rpc.Pkg{newPkg("dummy-3")},
			srcinfos:  map[string]*gosrc.Srcinfo{"dummy-3": makeSrcinfo("dummy-3", "11E521D646982372EB577A1F8F0871F202119294", "C52048C0C0748FEE227D47A2702353E0F7E48EDB")},
			bases:     map[string][]*rpc.Pkg{"dummy-3": {newPkg("dummy-3")}},
			wantError: false,
		},
		// Two dummy packages with existing keys.
		{
			pkgs:      []*rpc.Pkg{newPkg("dummy-4"), newPkg("dummy-5")},
			srcinfos:  map[string]*gosrc.Srcinfo{"dummy-4": makeSrcinfo("dummy-4", "11E521D646982372EB577A1F8F0871F202119294"), "dummy-5": makeSrcinfo("dummy-5", "C52048C0C0748FEE227D47A2702353E0F7E48EDB")},
			bases:     map[string][]*rpc.Pkg{"dummy-4": {newPkg("dummy-4")}, "dummy-5": {newPkg("dummy-5")}},
			wantError: false,
		},
		// Dummy package with invalid key, should fail.
		{
			pkgs:      []*rpc.Pkg{newPkg("dummy-7")},
			srcinfos:  map[string]*gosrc.Srcinfo{"dummy-7": makeSrcinfo("dummy-7", "THIS-SHOULD-FAIL")},
			bases:     map[string][]*rpc.Pkg{"dummy-7": {newPkg("dummy-7")}},
			wantError: true,
		},
		// Dummy package with both an invalid an another valid key, should fail.
		// A314827C4E4250A204CE6E13284FC34C8E4B1A25: Thomas Bächler.
		{
			pkgs:      []*rpc.Pkg{newPkg("dummy-8")},
			srcinfos:  map[string]*gosrc.Srcinfo{"dummy-8": makeSrcinfo("dummy-8", "A314827C4E4250A204CE6E13284FC34C8E4B1A25", "THIS-SHOULD-FAIL")},
			bases:     map[string][]*rpc.Pkg{"dummy-8": {newPkg("dummy-8")}},
			wantError: true,
		},
	}

	for _, tt := range casetests {
		err := checkPgpKeys(tt.pkgs, tt.bases, tt.srcinfos)
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
