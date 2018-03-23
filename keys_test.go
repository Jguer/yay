package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"testing"

	rpc "github.com/mikkeloscar/aur"
	gopkg "github.com/mikkeloscar/gopkgbuild"
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

func newSplitPkg(basename, name string) *rpc.Pkg {
	return &rpc.Pkg{Name: name, PackageBase: basename}
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

func TestFormatKeysToImport(t *testing.T) {
	casetests := []struct {
		keySet    pgpKeySet
		bases     map[string][]*rpc.Pkg
		expected  string
		alternate string
		wantError bool
	}{
		// Single key, required by single package.
		{
			keySet:    pgpKeySet{"KEY-1": []*rpc.Pkg{newPkg("PKG-foo")}},
			expected:  fmt.Sprintf("GPG keys need importing:\n\tKEY-1, required by: PKG-foo\n%s Import?", arrow),
			wantError: false,
		},
		// Single key, required by two packages.
		{
			keySet:    pgpKeySet{"KEY-1": []*rpc.Pkg{newPkg("PKG-foo"), newPkg("PKG-bar")}},
			expected:  fmt.Sprintf("GPG keys need importing:\n\tKEY-1, required by: PKG-foo PKG-bar\n%s Import?", arrow),
			wantError: false,
		},
		// Two keys, each required by a single package. Since iterating the map
		// does not force any particular order, we cannot really predict the
		// order in which the elements will appear. As we have only two cases,
		// let's add the second possibility to the alternate variable, to check
		// if there are any errors.
		{
			keySet:    pgpKeySet{"KEY-1": []*rpc.Pkg{newPkg("PKG-foo")}, "KEY-2": []*rpc.Pkg{newPkg("PKG-bar")}},
			expected:  fmt.Sprintf("GPG keys need importing:\n\tKEY-1, required by: PKG-foo\n\tKEY-2, required by: PKG-bar\n%s Import?", arrow),
			alternate: fmt.Sprintf("GPG keys need importing:\n\tKEY-2, required by: PKG-bar\n\tKEY-1, required by: PKG-foo\n%s Import?", arrow),
			wantError: false,
		},
		// Two keys required by single package.
		{
			keySet:    pgpKeySet{"KEY-1": []*rpc.Pkg{newPkg("PKG-foo")}, "KEY-2": []*rpc.Pkg{newPkg("PKG-foo")}},
			expected:  fmt.Sprintf("GPG keys need importing:\n\tKEY-1, required by: PKG-foo\n\tKEY-2, required by: PKG-foo\n%s Import?", arrow),
			alternate: fmt.Sprintf("GPG keys need importing:\n\tKEY-2, required by: PKG-foo\n\tKEY-1, required by: PKG-foo\n%s Import?", arrow),
			wantError: false,
		},
		// Two keys, one of them required by two packages.
		{
			keySet:    pgpKeySet{"KEY-1": []*rpc.Pkg{newPkg("PKG-foo"), newPkg("PKG-bar")}, "KEY-2": []*rpc.Pkg{newPkg("PKG-bar")}},
			expected:  fmt.Sprintf("GPG keys need importing:\n\tKEY-1, required by: PKG-foo PKG-bar\n\tKEY-2, required by: PKG-bar\n%s Import?", arrow),
			alternate: fmt.Sprintf("GPG keys need importing:\n\tKEY-2, required by: PKG-bar\n\tKEY-1, required by: PKG-foo PKG-bar\n%s Import?", arrow),
			wantError: false,
		},
		// Two keys, split package (linux-ck/linux-ck-headers).
		{
			keySet: pgpKeySet{"ABAF11C65A2970B130ABE3C479BE3E4300411886": []*rpc.Pkg{newPkg("linux-ck")}, "647F28654894E3BD457199BE38DBBDC86092693E": []*rpc.Pkg{newPkg("linux-ck")}},

			bases:     map[string][]*rpc.Pkg{"linux-ck": {newSplitPkg("linux-ck", "linux-ck-headers"), newPkg("linux-ck")}},
			expected:  fmt.Sprintf("GPG keys need importing:\n\tABAF11C65A2970B130ABE3C479BE3E4300411886, required by: linux-ck (linux-ck-headers linux-ck)\n\t647F28654894E3BD457199BE38DBBDC86092693E, required by: linux-ck (linux-ck-headers linux-ck)\n%s Import?", arrow),
			alternate: fmt.Sprintf("GPG keys need importing:\n\t647F28654894E3BD457199BE38DBBDC86092693E, required by: linux-ck (linux-ck-headers linux-ck)\n\tABAF11C65A2970B130ABE3C479BE3E4300411886, required by: linux-ck (linux-ck-headers linux-ck)\n%s Import?", arrow),
			wantError: false,
		},
		// One key, three split packages.
		{
			keySet:    pgpKeySet{"KEY-1": []*rpc.Pkg{newPkg("PKG-foo")}},
			bases:     map[string][]*rpc.Pkg{"PKG-foo": {newPkg("PKG-foo"), newSplitPkg("PKG-foo", "PKG-foo-1"), newSplitPkg("PKG-foo", "PKG-foo-2")}},
			expected:  fmt.Sprintf("GPG keys need importing:\n\tKEY-1, required by: PKG-foo (PKG-foo PKG-foo-1 PKG-foo-2)\n%s Import?", arrow),
			wantError: false,
		},
		// No keys, should fail.
		{
			keySet:    pgpKeySet{},
			expected:  "",
			wantError: true,
		},
	}

	for _, tt := range casetests {
		question, err := formatKeysToImport(tt.keySet, tt.bases)
		if !tt.wantError {
			if err != nil {
				t.Fatalf("Got error %q, want no error", err)
			}

			if question != tt.expected && question != tt.alternate {
				t.Fatalf("Got %q\n, expected: %q", question, tt.expected)
			}
			continue
		}
		// Here, we want to see the error.
		if err == nil {
			t.Fatalf("Got no error; want error")
		}
	}
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
	defer server.Shutdown(nil)

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

func TestCheckPgpKeys(t *testing.T) {
	keyringDir, err := ioutil.TempDir("/tmp", "yay-test-keyring")
	if err != nil {
		t.Fatalf("Unable to init test keyring: %v\n", err)
	}
	defer os.RemoveAll(keyringDir)

	config.GpgBin = "gpg"
	config.GpgFlags = fmt.Sprintf("--homedir %s --keyserver 127.0.0.1", keyringDir)

	server := startPgpKeyServer()
	defer server.Shutdown(nil)

	casetests := []struct {
		pkgs      []*rpc.Pkg
		srcinfos  map[string]*gopkg.PKGBUILD
		bases     map[string][]*rpc.Pkg
		wantError bool
	}{
		// cower: single package, one valid key not yet in the keyring.
		// 487EACC08557AD082088DABA1EB2638FF56C0C53: Dave Reisner.
		{
			pkgs:      []*rpc.Pkg{newPkg("cower")},
			srcinfos:  map[string]*gopkg.PKGBUILD{"cower": &gopkg.PKGBUILD{Pkgbase: "cower", Validpgpkeys: []string{"487EACC08557AD082088DABA1EB2638FF56C0C53"}}},
			bases:     map[string][]*rpc.Pkg{"cower": {newPkg("cower")}},
			wantError: false,
		},
		// libc++: single package, two valid keys not yet in the keyring.
		// 11E521D646982372EB577A1F8F0871F202119294: Tom Stellard.
		// B6C8F98282B944E3B0D5C2530FC3042E345AD05D: Hans Wennborg.
		{
			pkgs:      []*rpc.Pkg{newPkg("libc++")},
			srcinfos:  map[string]*gopkg.PKGBUILD{"libc++": &gopkg.PKGBUILD{Pkgbase: "libc++", Validpgpkeys: []string{"11E521D646982372EB577A1F8F0871F202119294", "B6C8F98282B944E3B0D5C2530FC3042E345AD05D"}}},
			bases:     map[string][]*rpc.Pkg{"libc++": {newPkg("libc++")}},
			wantError: false,
		},
		// Two dummy packages requiring the same key.
		// ABAF11C65A2970B130ABE3C479BE3E4300411886: Linus Torvalds.
		{
			pkgs:      []*rpc.Pkg{newPkg("dummy-1"), newPkg("dummy-2")},
			srcinfos:  map[string]*gopkg.PKGBUILD{"dummy-1": &gopkg.PKGBUILD{Pkgbase: "dummy-1", Validpgpkeys: []string{"ABAF11C65A2970B130ABE3C479BE3E4300411886"}}, "dummy-2": &gopkg.PKGBUILD{Pkgbase: "dummy-2", Validpgpkeys: []string{"ABAF11C65A2970B130ABE3C479BE3E4300411886"}}},
			bases:     map[string][]*rpc.Pkg{"dummy-1": {newPkg("dummy-1")}, "dummy-2": {newPkg("dummy-2")}},
			wantError: false,
		},
		// dummy package: single package, two valid keys, one of them already
		// in the keyring.
		// 11E521D646982372EB577A1F8F0871F202119294: Tom Stellard.
		// C52048C0C0748FEE227D47A2702353E0F7E48EDB: Thomas Dickey.
		{
			pkgs:      []*rpc.Pkg{newPkg("dummy-3")},
			srcinfos:  map[string]*gopkg.PKGBUILD{"dummy-3": &gopkg.PKGBUILD{Pkgbase: "dummy-3", Validpgpkeys: []string{"11E521D646982372EB577A1F8F0871F202119294", "C52048C0C0748FEE227D47A2702353E0F7E48EDB"}}},
			bases:     map[string][]*rpc.Pkg{"dummy-3": {newPkg("dummy-3")}},
			wantError: false,
		},
		// Two dummy packages with existing keys.
		{
			pkgs:      []*rpc.Pkg{newPkg("dummy-4"), newPkg("dummy-5")},
			srcinfos:  map[string]*gopkg.PKGBUILD{"dummy-4": &gopkg.PKGBUILD{Pkgbase: "dummy-4", Validpgpkeys: []string{"11E521D646982372EB577A1F8F0871F202119294"}}, "dummy-5": &gopkg.PKGBUILD{Pkgbase: "dummy-5", Validpgpkeys: []string{"C52048C0C0748FEE227D47A2702353E0F7E48EDB"}}},
			bases:     map[string][]*rpc.Pkg{"dummy-4": {newPkg("dummy-4")}, "dummy-5": {newPkg("dummy-5")}},
			wantError: false,
		},
		// Dummy package with invalid key, should fail.
		{
			pkgs:      []*rpc.Pkg{newPkg("dummy-7")},
			srcinfos:  map[string]*gopkg.PKGBUILD{"dummy-7": &gopkg.PKGBUILD{Pkgbase: "dummy-7", Validpgpkeys: []string{"THIS-SHOULD-FAIL"}}},
			bases:     map[string][]*rpc.Pkg{"dummy-7": {newPkg("dummy-7")}},
			wantError: true,
		},
		// Dummy package with both an invalid an another valid key, should fail.
		// A314827C4E4250A204CE6E13284FC34C8E4B1A25: Thomas Bächler.
		{
			pkgs:      []*rpc.Pkg{newPkg("dummy-8")},
			srcinfos:  map[string]*gopkg.PKGBUILD{"dummy-8": &gopkg.PKGBUILD{Pkgbase: "dummy-8", Validpgpkeys: []string{"A314827C4E4250A204CE6E13284FC34C8E4B1A25", "THIS-SHOULD-FAIL"}}},
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
