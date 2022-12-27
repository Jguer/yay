package pgp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	gosrc "github.com/Morganamilo/go-srcinfo"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v11/pkg/settings/exe"
	"github.com/Jguer/yay/v11/pkg/text"
)

// pgpKeySet maps a PGP key with a list of PKGBUILDs that require it.
// This is similar to stringSet, used throughout the code.
type pgpKeySet map[string][]string

func (set pgpKeySet) toSlice() []string {
	slice := make([]string, 0, len(set))
	for v := range set {
		slice = append(slice, v)
	}

	return slice
}

func (set pgpKeySet) set(key, p string) {
	// Using ToUpper to make sure keys with a different case will be
	// considered the same.
	upperKey := strings.ToUpper(key)
	set[upperKey] = append(set[upperKey], p)
}

func (set pgpKeySet) get(key string) bool {
	upperKey := strings.ToUpper(key)
	_, exists := set[upperKey]

	return exists
}

type GPGCmdBuilder interface {
	exe.Runner
	BuildGPGCmd(ctx context.Context, extraArgs ...string) *exec.Cmd
}

// CheckPgpKeys iterates through the keys listed in the PKGBUILDs and if needed,
// asks the user whether yay should try to import them.
func CheckPgpKeys(ctx context.Context, pkgbuildDirsByBase map[string]string, srcinfos map[string]*gosrc.Srcinfo,
	cmdBuilder GPGCmdBuilder, noConfirm bool,
) ([]string, error) {
	// Let's check the keys individually, and then we can offer to import
	// the problematic ones.
	problematic := make(pgpKeySet)

	// Mapping all the keys.
	for pkg := range pkgbuildDirsByBase {
		srcinfo := srcinfos[pkg]

		for _, key := range srcinfo.ValidPGPKeys {
			// If key already marked as problematic, indicate the current
			// PKGBUILD requires it.
			if problematic.get(key) {
				problematic.set(key, pkg)
				continue
			}

			if err := cmdBuilder.Show(cmdBuilder.BuildGPGCmd(ctx, "--list-keys", key)); err != nil {
				problematic.set(key, pkg)
			}
		}
	}

	// No key issues!
	if len(problematic) == 0 {
		return []string{}, nil
	}

	str, err := formatKeysToImport(problematic)
	if err != nil {
		return nil, err
	}

	fmt.Println()
	fmt.Println(str)

	if text.ContinueTask(os.Stdin, gotext.Get("Import?"), true, noConfirm) {
		return problematic.toSlice(), importKeys(ctx, cmdBuilder, problematic.toSlice())
	}

	return problematic.toSlice(), nil
}

// importKeys tries to import the list of keys specified in its argument.
func importKeys(ctx context.Context, cmdBuilder GPGCmdBuilder, keys []string) error {
	text.OperationInfoln(gotext.Get("Importing keys with gpg..."))

	if err := cmdBuilder.Show(cmdBuilder.BuildGPGCmd(ctx, append([]string{"--recv-keys"}, keys...)...)); err != nil {
		return errors.New(gotext.Get("problem importing keys"))
	}

	return nil
}

// formatKeysToImport receives a set of keys and returns a string containing the
// question asking the user wants to import the problematic keys.
func formatKeysToImport(keys pgpKeySet) (string, error) {
	if len(keys) == 0 {
		return "", errors.New(gotext.Get("no keys to import"))
	}

	var buffer bytes.Buffer

	buffer.WriteString(text.SprintOperationInfo(gotext.Get("PGP keys need importing:")))

	for key, bases := range keys {
		pkglist := ""
		for _, base := range bases {
			pkglist += base + "  "
		}

		pkglist = strings.TrimRight(pkglist, " ")
		buffer.WriteString("\n" + text.SprintWarn(gotext.Get("%s, required by: %s", text.Cyan(key), text.Cyan(pkglist))))
	}

	return buffer.String(), nil
}
