package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	gosrc "github.com/Morganamilo/go-srcinfo"
	rpc "github.com/mikkeloscar/aur"
)

// pgpKeySet maps a PGP key with a list of PKGBUILDs that require it.
// This is similar to stringSet, used throughout the code.
type pgpKeySet map[string][]*rpc.Pkg

func (set pgpKeySet) toSlice() []string {
	slice := make([]string, 0, len(set))
	for v := range set {
		slice = append(slice, v)
	}
	return slice
}

func (set pgpKeySet) set(key string, p *rpc.Pkg) {
	// Using ToUpper to make sure keys with a different case will be
	// considered the same.
	upperKey := strings.ToUpper(key)
	if _, exists := set[upperKey]; !exists {
		set[upperKey] = []*rpc.Pkg{}
	}
	set[key] = append(set[key], p)
}

func (set pgpKeySet) get(key string) bool {
	upperKey := strings.ToUpper(key)
	_, exists := set[upperKey]
	return exists
}

// checkPgpKeys iterates through the keys listed in the PKGBUILDs and if needed,
// asks the user whether yay should try to import them.
func checkPgpKeys(pkgs []*rpc.Pkg, bases map[string][]*rpc.Pkg, srcinfos map[string]*gosrc.Srcinfo) error {
	// Let's check the keys individually, and then we can offer to import
	// the problematic ones.
	problematic := make(pgpKeySet)
	args := append(strings.Fields(config.GpgFlags), "--list-keys")

	// Mapping all the keys.
	for _, pkg := range pkgs {
		srcinfo := srcinfos[pkg.PackageBase]

		for _, key := range srcinfo.ValidPGPKeys {
			// If key already marked as problematic, indicate the current
			// PKGBUILD requires it.
			if problematic.get(key) {
				problematic.set(key, pkg)
				continue
			}

			cmd := exec.Command(config.GpgBin, append(args, key)...)
			err := cmd.Run()
			if err != nil {
				problematic.set(key, pkg)
			}
		}
	}

	// No key issues!
	if len(problematic) == 0 {
		return nil
	}

	str, err := formatKeysToImport(problematic, bases)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println(str)

	if continueTask(bold(green("Import?")), true) {
		return importKeys(problematic.toSlice())
	}

	return nil
}

// importKeys tries to import the list of keys specified in its argument.
func importKeys(keys []string) error {
	args := append(strings.Fields(config.GpgFlags), "--recv-keys")
	cmd := exec.Command(config.GpgBin, append(args, keys...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

	fmt.Printf("%s %s...\n", bold(cyan("::")), bold("Importing keys with gpg..."))
	err := cmd.Run()

	if err != nil {
		return fmt.Errorf("%s Problem importing keys", bold(red(arrow+" Error:")))
	}
	return nil
}

// formatKeysToImport receives a set of keys and returns a string containing the
// question asking the user wants to import the problematic keys.
func formatKeysToImport(keys pgpKeySet, bases map[string][]*rpc.Pkg) (string, error) {
	if len(keys) == 0 {
		return "", fmt.Errorf("%s No keys to import", bold(red(arrow+" Error:")))
	}

	var buffer bytes.Buffer
	buffer.WriteString(bold(green(arrow)))
	buffer.WriteString(bold(green(" PGP keys need importing:")))
	for key, pkgs := range keys {
		pkglist := ""
		for _, pkg := range pkgs {
			pkglist += formatPkgbase(bases[pkg.PackageBase]) + " "
		}
		pkglist = strings.TrimRight(pkglist, " ")
		buffer.WriteString(fmt.Sprintf("\n%s %s, required by: %s", yellow(bold(smallArrow)), cyan(key), cyan(pkglist)))
	}
	return buffer.String(), nil
}
