package main

// GetPkgbuild gets the pkgbuild of the package 'pkg' trying the ABS first and then the AUR trying the ABS first and then the AUR.

// RemovePackage removes package from VCS information
func removeVCSPackage(pkgs []string) {
	updated := false

	for _, pkgName := range pkgs {
		_, ok := savedInfo[pkgName]
		if ok {
			delete(savedInfo, pkgName)
			updated = true
		}
	}

	if updated {
		saveVCSInfo()
	}
}

// CleanDependencies removes all dangling dependencies in system
func cleanDependencies() error {
	hanging, err := hangingPackages()
	if err != nil {
		return err
	}

	if len(hanging) != 0 {
		if !continueTask("Confirm Removal?", "nN") {
			return nil
		}
		err = cleanRemove(hanging)
	}

	return err
}

// CleanRemove sends a full removal command to pacman with the pkgName slice
func cleanRemove(pkgNames []string) (err error) {
	if len(pkgNames) == 0 {
		return nil
	}

	oldvalue := config.NoConfirm
	config.NoConfirm = true
	arguments := makeArguments()
	arguments.addArg("R")
	arguments.addTarget(pkgNames...)
	err = passToPacman(arguments)
	config.NoConfirm = oldvalue
	return err
}
