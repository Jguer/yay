package pkgbuildrepo

import (
	"fmt"
	"path/filepath"
	"strings"

	gosrc "github.com/Morganamilo/go-srcinfo"
)

// Entry is one PKGBUILD found in a repository, keyed by its package base.
type Entry struct {
	RepoName string
	Dir      string
	Pkgbase  string
	Version  string
	Srcinfo  *gosrc.Srcinfo
}

// Index resolves package names to the PKGBUILD-repo entry that provides them.
// Real package names (pkgbase/pkgname) always take priority over provides,
// regardless of the order repos are added.
type Index struct {
	byName     map[string]*Entry
	byProvides map[string]*Entry
}

func NewIndex() *Index {
	return &Index{
		byName:     map[string]*Entry{},
		byProvides: map[string]*Entry{},
	}
}

// AddRepo parses the .SRCINFO of each dir and adds its packages to the index.
// The first entry to claim a given name wins, so earlier repos mask later ones.
func (i *Index) AddRepo(repoName string, dirs []string) error {
	for _, dir := range dirs {
		si, err := gosrc.ParseFile(filepath.Join(dir, ".SRCINFO"))
		if err != nil {
			return fmt.Errorf("parsing %s: %w", filepath.Join(dir, ".SRCINFO"), err)
		}

		entry := &Entry{
			RepoName: repoName,
			Dir:      dir,
			Pkgbase:  si.Pkgbase,
			Version:  si.Version(),
			Srcinfo:  si,
		}

		i.claimName(si.Pkgbase, entry)
		for _, pkg := range si.SplitPackages() {
			i.claimName(pkg.Pkgname, entry)
			for _, prov := range pkg.Provides {
				i.claimProvide(provideName(prov.Value), entry)
			}
		}
	}

	return nil
}

// Get resolves a package name to its entry, checking real names before
// provides.
func (i *Index) Get(name string) (*Entry, bool) {
	if e, ok := i.byName[name]; ok {
		return e, true
	}

	if e, ok := i.byProvides[name]; ok {
		return e, true
	}

	return nil, false
}

func (i *Index) claimName(name string, entry *Entry) {
	if name == "" {
		return
	}

	if _, exists := i.byName[name]; !exists {
		i.byName[name] = entry
	}
}

func (i *Index) claimProvide(name string, entry *Entry) {
	if name == "" {
		return
	}

	if _, exists := i.byProvides[name]; !exists {
		i.byProvides[name] = entry
	}
}

// provideName strips any version constraint from a provides value, e.g.
// "libfoo.so=1" or "bar>=2.0" becomes the bare package name.
func provideName(value string) string {
	if i := strings.IndexAny(value, "=<>"); i >= 0 {
		return value[:i]
	}

	return value
}
