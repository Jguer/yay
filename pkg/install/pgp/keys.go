package pgp

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Jguer/yay/v10/pkg/text"
	"github.com/Jguer/yay/v10/pkg/types"
	gosrc "github.com/Morganamilo/go-srcinfo"
)

const smallArrow = " ->"
const arrow = "==>"

// KeySet maps a PGP key with a list of PKGBUILDs that require it.
// This is similar to stringSet, used throughout the code.
type keySet map[string][]types.Base

func (set keySet) toSlice() []string {
	slice := make([]string, 0, len(set))
	for v := range set {
		slice = append(slice, v)
	}
	return slice
}

func (set keySet) set(key string, p types.Base) {
	// Using ToUpper to make sure keys with a different case will be
	// considered the same.
	upperKey := strings.ToUpper(key)
	set[key] = append(set[upperKey], p)
}

func (set keySet) get(key string) bool {
	upperKey := strings.ToUpper(key)
	_, exists := set[upperKey]
	return exists
}

// CheckKeys iterates through the keys listed in the PKGBUILDs and if needed,
// asks the user whether yay should try to import them.
func CheckKeys(gpgBin string, gpgFlags string, bases []types.Base, srcinfos map[string]*gosrc.Srcinfo, noConfirm bool) error {
	// Let's check the keys individually, and then we can offer to import
	// the problematic ones.
	problematic := make(keySet)
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

	if text.ContinueTask(text.Bold(text.Green("Import?")), true, noConfirm) {
		return importKeys(gpgBin, gpgFlags, problematic.toSlice())
	}

	return nil
}

// importKeys tries to import the list of keys specified in its argument.
func importKeys(gpgBin string, gpgFlags string, keys []string) error {
	args := append(strings.Fields(gpgFlags), "--recv-keys") // caller already fields this, maybe pass that reference instead.
	cmd := exec.Command(gpgBin, append(args, keys...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

	fmt.Printf("%s %s...\n", text.Bold(text.Cyan("::")), text.Bold("Importing keys with gpg..."))
	err := cmd.Run()

	if err != nil {
		return fmt.Errorf("%s Problem importing keys", text.Bold(text.Red(arrow+" Error:")))
	}
	return nil
}

// formatKeysToImport receives a set of keys and returns a string containing the
// question asking the user wants to import the problematic keys.
func formatKeysToImport(keys keySet) (string, error) {
	if len(keys) == 0 {
		return "", fmt.Errorf("%s No keys to import", text.Bold(text.Red(arrow+" Error:")))
	}

	var buffer bytes.Buffer
	buffer.WriteString(text.Bold(text.Green(arrow)))
	buffer.WriteString(text.Bold(text.Green(" PGP keys need importing:")))
	for key, bases := range keys {
		pkglist := ""
		for _, base := range bases {
			pkglist += base.String() + "  "
		}
		pkglist = strings.TrimRight(pkglist, " ")
		buffer.WriteString(fmt.Sprintf("\n%s %s, required by: %s", text.Yellow(text.Bold(smallArrow)), text.Cyan(key), text.Cyan(pkglist)))
	}
	return buffer.String(), nil
}
