package completion

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	alpm "github.com/Jguer/go-alpm"
)

// Show provides completion info for shells
func Show(alpmHandle *alpm.Handle, aurURL, completionPath string, interval int, force bool) error {
	err := Update(alpmHandle, aurURL, completionPath, interval, force)
	if err != nil {
		return err
	}

	in, err := os.OpenFile(completionPath, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	defer in.Close()

	_, err = io.Copy(os.Stdout, in)
	return err
}

// Update updates completion cache to be used by Complete
func Update(alpmHandle *alpm.Handle, aurURL, completionPath string, interval int, force bool) error {
	info, err := os.Stat(completionPath)

	if os.IsNotExist(err) || (interval != -1 && time.Since(info.ModTime()).Hours() >= float64(interval*24)) || force {
		errd := os.MkdirAll(filepath.Dir(completionPath), 0o755)
		if errd != nil {
			return errd
		}
		out, errf := os.Create(completionPath)
		if errf != nil {
			return errf
		}

		if createAURList(aurURL, out) != nil {
			defer os.Remove(completionPath)
		}

		dbList, err := alpmHandle.SyncDBs()
		if err != nil {
			return err
		}
		erra := createRepoList(&dbList, out)

		out.Close()
		return erra
	}

	return nil
}

// CreateAURList creates a new completion file
func createAURList(aurURL string, out io.Writer) error {
	u, err := url.Parse(aurURL)
	if err != nil {
		return err
	}
	u.Path = path.Join(u.Path, "packages.gz")
	resp, err := http.Get(u.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid status code: %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)

	scanner.Scan()
	for scanner.Scan() {
		text := scanner.Text()
		if strings.HasPrefix(text, "#") {
			continue
		}
		_, err = io.WriteString(out, text+"\tAUR\n")
		if err != nil {
			return err
		}
	}

	return nil
}

// CreatePackageList appends Repo packages to completion cache
func createRepoList(dbList *alpm.DBList, out io.Writer) error {
	_ = dbList.ForEach(func(db alpm.DB) error {
		_ = db.PkgCache().ForEach(func(pkg alpm.Package) error {
			_, err := io.WriteString(out, pkg.Name()+"\t"+pkg.DB().Name()+"\n")
			return err
		})
		return nil
	})
	return nil
}
