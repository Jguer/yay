// aurfetch is a library for managing the downloading, updating and reviewing of packages
// downloaded from the AUR.
//
// The general workflow is:
//	// Create handle
//	fetch := aurfetch.MakeHandle()
//	pkgs := []string{"foo", "bar"}
//
//	// Download pkgs
//	fetched, err := fetch.Download(pkgs)
//	// Filter to packages that need to be merged
//	toMerge, err := fetch.NeedsMerge(fetched)
//
//	// Insert review code with either fetch.PrintDiffs or fetch.DiffsToFile or fetch.MakeView
//
//	// Merge
//	err := fetch.Merge(toMerge)
//	// Mark everything as seen
//	err := fetch.MarkSeen(pkgs)
package aurfetch

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
)

// DownloadCB is the callback passed into the download function.
// pkg: The pkgbase of the package being downloaded
// n: The number of packages that have been downloaded
// out: the output from git clone/fetch
type DownloadCB func(pkg string, n int, out string, err error)

// MergeCB is the callback passed into the merge function.
// pkg: The pkgbase of the package being merged
// n: The number of packages that have been merged
type MergeCB func(pkg string, n int, out string)

// URL gets the URL to the specified AUR git repo
func (h Handle) URL(pkgbase string) string {
	return h.AURURL + "/" + pkgbase + ".git"
}

// Download downloads a list of packages. git clone will be used for
// new downloads and git fetch for existing rpos
func (h Handle) Download(pkgs []string) ([]string, *MultiError) {
	err := MultiError{}

	cb := func(pkg string, n int, out string, e error) {
		if e != nil {
			err.Add(e)
		}
	}

	fetched := h.DownloadCB(pkgs, cb)
	return fetched, err.Return()
}

// DownloadCB downloads a list of packages and calls cb after each download.
// git clone will be used for new downloads and git fetch for existing rpos
func (h Handle) DownloadCB(pkgs []string, cb DownloadCB) []string {
	var wg sync.WaitGroup
	var mux sync.Mutex
	fetched := make([]string, 0)
	n := 0

	os.MkdirAll(h.CacheDir, os.ModePerm)

	f := func(pkg string) {
		defer wg.Done()
		out, clone, e := h.gitDownload(h.URL(pkg), h.CacheDir, pkg)

		mux.Lock()
		n++
		if !clone {
			fetched = append(fetched, pkg)
		}
		cb(pkg, n, out, e)
		mux.Unlock()
	}

	for _, pkg := range pkgs {
		wg.Add(1)
		go f(pkg)
	}

	wg.Wait()

	return fetched
}

// NeedsMerge filters a list of packages to ones that need to be merged.
//
// A package is considered to need merging if AUR_SEEN is equal or newer than upstream's HEAD
// If AUR_SEEN is not defined then HEAD is used instead.
func (h Handle) NeedsMerge(pkgs []string) ([]string, error) {
	toMerge := make([]string, 0)

	for _, pkg := range pkgs {
		if h.gitNeedMerge(h.CacheDir, pkg) {
			toMerge = append(toMerge, pkg)
		}
	}

	return toMerge, nil
}

// Merge merges a list of packages with upstream.
func (h Handle) Merge(pkgs []string) error {
	cb := func(pkg string, n int, out string) {}
	return h.MergeCB(pkgs, cb)
}

// MergeCB merges a list of packages with upstram, calling cb after each successful merge.
func (h Handle) MergeCB(pkgs []string, cb MergeCB) error {
	for n, pkg := range pkgs {
		out, err := h.gitMerge(h.CacheDir, pkg)
		n += 1

		if err != nil {
			return err
		}

		cb(pkg, n, out)
	}

	return nil
}

// MarkSeen marks a packages as seen.
//
// If a package has been seen then it is assumed everything up to and including
// the current commit has been reviewed by the user. Therefore diffs will be between
// the repo at this point and upstream.
func (h Handle) MarkSeen(pkgs []string) error {
	for _, pkg := range pkgs {
		err := h.gitUpdateRef(h.CacheDir, pkg)
		if err != nil {
			return err
		}
	}

	return nil
}

// PrintDiffs prints a list of diffs using git diff.
// This means git's pager and other config settings will be respected
func (h Handle) PrintDiffs(pkgs []string) error {
	for _, pkg := range pkgs {
		err := h.gitPrintDiff(h.CacheDir, pkg)
		if err != nil {
			return err
		}
	}

	return nil
}

// DiffsToFile writes diffs of packages to DiffDir.
//  Each diff will be named <pkgbase>.diff
func (h Handle) DiffsToFile(pkgs []string, colour bool) error {
	err := os.MkdirAll(h.PatchDir, os.ModePerm)
	if err != nil {
		return nil
	}

	for _, pkg := range pkgs {
		out, err := h.gitDiff(h.CacheDir, colour, pkg)
		if err != nil {
			return err
		}

		err = ioutil.WriteFile(filepath.Join(h.PatchDir, pkg)+".diff", []byte(out), 0644)

		if err != nil {
			return err
		}
	}

	return nil
}

func (h Handle) linkPkgs(pkgs []string, tmp string) error {
	for _, pkg := range pkgs {
		path := filepath.Join(h.CacheDir, pkg)
		tmpPath := filepath.Join(tmp, pkg)
		_, err := os.Stat(path)

		if err != nil {
			return err
		}

		err = os.Symlink(path, tmpPath)
		if err != nil {
			return err
		}

		path = filepath.Join(h.CacheDir, pkg, "PKGBUILD")
		tmpPath = filepath.Join(tmp, pkg+"-"+"PKGBUILD")
		_, err = os.Stat(path)

		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}

		err = os.Symlink(path, tmpPath)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	return nil
}

func (h Handle) linkDiffs(diffs []string, tmp string) error {
	for _, pkg := range diffs {
		path := filepath.Join(h.PatchDir, pkg) + ".diff"
		tmpPath := filepath.Join(tmp, pkg) + ".diff"
		_, err := os.Stat(path)

		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}

		err = os.Symlink(path, tmpPath)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	return nil
}

// Creates a view of packages to be installed.
//
// A view is intended for the user to inspect for security purposes.
// The view will include pkgbuilds of newly cloned packages and diffs of
// packages that have been updated. There is also a directory containing all
// the packages in the transaction.
//
// The returning string is a path to the tempdir created to host this view.
// The directory should be deleted by the caller when the view is no longer needed.
//
// Note the diffs should have already been created using DiffsToFile before calling.
func (h Handle) MakeView(pkgs []string, diffs []string) (string, error) {
	tmp, err := ioutil.TempDir("", "aur.")
	if err != nil {
		return "", nil
	}

	newPkgs := []string{}

	err = os.MkdirAll(h.CacheDir, os.ModePerm)
	if err != nil {
		goto err
	}

	for _, pkg := range pkgs {
		if !h.gitHasRef(h.CacheDir, pkg) {
			newPkgs = append(newPkgs, pkg)
		}
	}

	if len(newPkgs) > 0 {
		err = h.linkPkgs(newPkgs, tmp)
		if err != nil {
			goto err
		}
	}

	if len(pkgs) > 0 {
		err = os.MkdirAll(filepath.Join(tmp, "-all"), os.ModePerm)
		if err != nil {
			goto err
		}

		h.linkPkgs(pkgs, filepath.Join(tmp, "-all"))
		if err != nil {
			goto err
		}
	}

	if len(diffs) > 0 {
		h.linkDiffs(pkgs, tmp)
		if err != nil {
			goto err
		}
	}

	return tmp, nil

err:
	os.RemoveAll(tmp)
	return "", fmt.Errorf("Failed to link build files: %s", err.Error())
}
