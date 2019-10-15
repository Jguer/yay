package completion

import (
	"bufio"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	alpm "github.com/Jguer/go-alpm"
)

// Show provides completion info for shells
func Show(alpmHandle *alpm.Handle, aurURL string, cacheDir string, interval int, force bool) error {
	path := filepath.Join(cacheDir, "completion.cache")

	err := Update(alpmHandle, aurURL, cacheDir, interval, force)
	if err != nil {
		return err
	}

	in, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer in.Close()

	_, err = io.Copy(os.Stdout, in)
	return err
}

// Update updates completion cache to be used by Complete
func Update(alpmHandle *alpm.Handle, aurURL string, cacheDir string, interval int, force bool) error {
	path := filepath.Join(cacheDir, "completion.cache")
	info, err := os.Stat(path)

	if os.IsNotExist(err) || (interval != -1 && time.Since(info.ModTime()).Hours() >= float64(interval*24)) || force {
		errd := os.MkdirAll(filepath.Dir(path), 0755)
		if errd != nil {
			return errd
		}
		out, errf := os.Create(path)
		if errf != nil {
			return errf
		}

		if createAURList(aurURL, out) != nil {
			defer os.Remove(path)
		}
		erra := createRepoList(alpmHandle, out)

		out.Close()
		return erra
	}

	return nil
}

//CreateAURList creates a new completion file
func createAURList(aurURL string, out io.Writer) error {
	resp, err := http.Get(aurURL + "/packages.gz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)

	scanner.Scan()
	for scanner.Scan() {
		_, err = io.WriteString(out, scanner.Text()+"\tAUR\n")
		if err != nil {
			return err
		}
	}

	return nil
}

//CreatePackageList appends Repo packages to completion cache
func createRepoList(alpmHandle *alpm.Handle, out io.Writer) error {
	dbList, err := alpmHandle.SyncDBs()
	if err != nil {
		return err
	}

	_ = dbList.ForEach(func(db alpm.DB) error {
		_ = db.PkgCache().ForEach(func(pkg alpm.Package) error {
			_, err = io.WriteString(out, pkg.Name()+"\t"+pkg.DB().Name()+"\n")
			return err
		})
		return nil
	})
	return nil
}
