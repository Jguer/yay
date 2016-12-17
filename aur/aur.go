package aur

import (
	"fmt"

	"github.com/jguer/yay/pacman"
)

// Install sends system commands to make and install a package from pkgName
func Install(pkg string, flags []string) (err error) {
	q, n, err := Info(pkg)
	if err != nil {
		return
	}

	if n == 0 {
		return fmt.Errorf("Package %s does not exist", pkg)
	}

	q[0].Install(flags)
	return err
}

// Upgrade tries to update every foreign package installed in the system
func Upgrade(flags []string) error {
	fmt.Println("\x1b[1;36;1m::\x1b[0m\x1b[1m Starting AUR upgrade...\x1b[0m")

	foreign, n, err := pacman.ForeignPackages()
	if err != nil || n == 0 {
		return err
	}

	keys := make([]string, len(foreign))
	i := 0
	for k := range foreign {
		keys[i] = k
		i++
	}

	q, _, err := MultiInfo(keys)
	if err != nil {
		return err
	}

	outdated := q[:0]
	for _, res := range q {
		if _, ok := foreign[res.Name]; ok {
			// Leaving this here for now, warn about downgrades later
			if res.LastModified > foreign[res.Name].Date {
				fmt.Printf("\x1b[1m\x1b[32m==>\x1b[33;1m %s: \x1b[0m%s \x1b[33;1m-> \x1b[0m%s\n",
					res.Name, foreign[res.Name].Version, res.Version)
				outdated = append(outdated, res)
			}
		}
	}

	//If there are no outdated packages, don't prompt
	if len(outdated) == 0 {
		fmt.Println(" there is nothing to do")
		return nil
	}

	// Install updated packages
	if !continueTask("Proceed with upgrade?", "nN") {
		return nil
	}

	for _, pkg := range outdated {
		pkg.Install(flags)
	}

	return nil
}
