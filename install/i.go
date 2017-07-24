package install

import "fmt"

// Install sends system commands to make and install a package from pkgName
func Install(pkgName []string, flags []string) (err error) {
	q, err := rpc.Info(pkgName)
	if err != nil {
		return
	}

	if len(q) != len(pkgName) {
		fmt.Printf("Some package from list\n%+v\ndoes not exist", pkgName)
	}

	var finalrm []string
	for _, i := range q {
		mrm, err := PkgInstall(&i, flags)
		if err != nil {
			fmt.Println("Error installing", i.Name, ":", err)
		}
		finalrm = append(finalrm, mrm...)
	}

	if len(finalrm) != 0 {
		err = RemoveMakeDeps(finalrm)
	}

	return err
}

// PkgInstall handles install from Info Result.
func PkgInstall(a []*rpc.Pkg, flags []string) (finalmdeps []string, err error) {
	for _, pkg := range a {
		if pkg.Maintainer == "" {
			fmt.Println("\x1b[1;31;40m==> Warning:\x1b[0;;40m This package is orphaned.\x1b[0m")
		}
	}
}
