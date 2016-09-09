package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func installnumArray(num []int, aurRes AurSearch, repoRes RepoSearch, flags ...string) (err error) {
	if len(num) == 0 {
		return errors.New("Installing AUR array: No nums selected")
	}

	var index int
	for _, i := range num {
		if i > repoRes.Resultcount-1 {
			index = i - repoRes.Resultcount
			err = aurRes.Results[i-index].install(flags...)
			if err != nil {
				// Do not abandon program, we might still be able to install the rest
				fmt.Println(err)
			}
		} else {
			InstallPackage(repoRes.Results[i].Name, flags...)
		}
	}

	return err
}

func installAURPackage(pkg string, flags ...string) (err error) {
	info, err := infoAurPackage(pkg)
	if err != nil {
		return
	}

	if info.Resultcount == 0 {
		return errors.New("Package '" + pkg + "' does not exist")
	}

	info.Results[0].install(flags...)
	return err
}

func (a AurResult) install(flags ...string) (err error) {
	// No need to use filepath.separators because it won't run on inferior platforms
	err = os.MkdirAll(BuildDir+"builds", 0755)
	if err != nil {
		fmt.Println(err)
		return
	}

	tarLocation := BuildDir + a.Name + ".tar.gz"
	defer os.Remove(BuildDir + a.Name + ".tar.gz")

	err = downloadFile(tarLocation, BaseURL+a.URLPath)
	if err != nil {
		return
	}

	err = exec.Command(TarBin, "-xf", tarLocation, "-C", BuildDir).Run()
	if err != nil {
		return
	}
	defer os.RemoveAll(BuildDir + a.Name)
	err = a.getAURDependencies()
	if err != nil {
		return
	}

	fmt.Print("\033[1m\x1b[32m==> Edit PKGBUILD? (y/n)\033[0m")
	var response string
	fmt.Scanln(&response)
	if strings.ContainsAny(response, "y & Y") {
		editcmd := exec.Command(Editor, BuildDir+a.Name+"/"+"PKGBUILD")
		editcmd.Stdout = os.Stdout
		editcmd.Stderr = os.Stderr
		editcmd.Stdin = os.Stdin
		err = editcmd.Run()
	}

	err = os.Chdir(BuildDir + a.Name)
	if err != nil {
		return
	}
	var args string
	if len(flags) != 0 {
		args = fmt.Sprintf(" %s", strings.Join(flags, " "))
	}
	makepkgcmd := exec.Command(MakepkgBin, "-sri"+args)
	makepkgcmd.Stdout = os.Stdout
	makepkgcmd.Stderr = os.Stderr
	makepkgcmd.Stdin = os.Stdin
	err = makepkgcmd.Run()

	return
}

// InstallPackage handles repo installs
func InstallPackage(pkg string, flags ...string) (err error) {
	var args string
	fmt.Println(len(flags))
	if len(flags) != 0 {
		args = fmt.Sprintf(" %s", strings.Join(flags, " "))
	}
	cmd := exec.Command("sudo", "pacman", "-S", pkg+args)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	return nil
}
