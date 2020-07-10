package pgp

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	gosrc "github.com/Morganamilo/go-srcinfo"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/dep"
	"github.com/Jguer/yay/v10/pkg/text"
)

// pgpKeySet maps a PGP key with a list of PKGBUILDs that require it.
// This is similar to stringSet, used throughout the code.
type pgpKeySet map[string][]dep.Base

func (set pgpKeySet) toSlice() []string {
	slice := make([]string, 0, len(set))
	for v := range set {
		slice = append(slice, v)
	}
	return slice
}

func (set pgpKeySet) set(key string, p dep.Base) {
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

// CheckPgpKeys iterates through the keys listed in the PKGBUILDs and if needed,
// asks the user whether yay should try to import them.
func CheckPgpKeys(bases []dep.Base, srcinfos map[string]*gosrc.Srcinfo,
	gpgBin, gpgFlags string, noConfirm bool) error {
	// Let's check the keys individually, and then we can offer to import
	// the problematic ones.
	problematic := make(pgpKeySet)
	args := append(strings.Fields(gpgFlags), "--list-keys")

	// Mapping all the keys.
	for _, base := range bases {
		pkg := base.Pkgbase()
		srcinfo := srcinfos[pkg]

		for _, key := range srcinfo.ValidPGPKeys {
			// If key already marked as problematic, indicate the current
			// PKGBUILD requires it.
			if problematic.get(key) {
				problematic.set(key, base)
				continue
			}

			cmd := exec.Command(gpgBin, append(args, key)...)
			err := cmd.Run()
			if err != nil {
				problematic.set(key, base)
			}
		}
	}

	// No key issues!
	if len(problematic) == 0 {
		return nil
	}

	str, err := formatKeysToImport(problematic)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println(str)

	if text.ContinueTask(gotext.Get("Import?"), true, noConfirm) {
		return importKeys(problematic.toSlice(), gpgBin, gpgFlags)
	}

	return nil
}

// importKeys tries to import the list of keys specified in its argument.
func importKeys(keys []string, gpgBin, gpgFlags string) error {
	args := append(strings.Fields(gpgFlags), "--recv-keys")
	cmd := exec.Command(gpgBin, append(args, keys...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

	text.OperationInfoln(gotext.Get("Importing keys with gpg..."))
	err := cmd.Run()
	if err != nil {
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
			pkglist += base.String() + "  "
		}
		pkglist = strings.TrimRight(pkglist, " ")
		buffer.WriteString("\n" + text.SprintWarn(gotext.Get("%s, required by: %s", text.Cyan(key), text.Cyan(pkglist))))
	}
	return buffer.String(), nil
}
