package aursource

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// PinFileName is the name of the state file, stored next to vcs.json, that
// remembers which AUR source each package was installed from.
const PinFileName = "aursources.json"

// PinFile maps installed package names to the name of the AUR source they
// were installed from. Packages installed from the first (default) source are
// not listed.
type PinFile struct {
	Packages map[string]string `json:"packages"`
}

// LoadPinFile reads path. A missing file is an empty PinFile.
func LoadPinFile(path string) (*PinFile, error) {
	file := &PinFile{Packages: map[string]string{}}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return file, nil
	}

	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, file); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	if file.Packages == nil {
		file.Packages = map[string]string{}
	}

	return file, nil
}

// Save writes the file atomically. An empty PinFile is not created when the
// file does not exist yet.
func (f *PinFile) Save(path string) error {
	if len(f.Packages) == 0 {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			return nil
		}
	}

	data, err := json.MarshalIndent(f, "", "\t")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}

	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}

	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmp.Name(), path)
}

// Names returns the pinned package names in order.
func (f *PinFile) Names() []string {
	names := make([]string, 0, len(f.Packages))
	for name := range f.Packages {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

// Record sets the source of pkgName; an empty source (the default source)
// removes the pin. It reports whether anything changed.
func (f *PinFile) Record(pkgName, source string) bool {
	prev, ok := f.Packages[pkgName]
	if source == "" {
		delete(f.Packages, pkgName)
		return ok
	}

	f.Packages[pkgName] = source

	return !ok || prev != source
}

// Prune drops packages for which installed returns false and returns them.
func (f *PinFile) Prune(installed func(string) bool) []string {
	var dropped []string

	for _, name := range f.Names() {
		if !installed(name) {
			delete(f.Packages, name)
			dropped = append(dropped, name)
		}
	}

	return dropped
}

// RenameSource moves every pin from source from to source to and returns the
// affected packages.
func (f *PinFile) RenameSource(from, to string) []string {
	var changed []string

	for _, name := range f.Names() {
		if f.Packages[name] == from {
			f.Packages[name] = to
			changed = append(changed, name)
		}
	}

	return changed
}

// DropSource removes every pin to source and returns the affected packages.
func (f *PinFile) DropSource(source string) []string {
	var dropped []string

	for _, name := range f.Names() {
		if f.Packages[name] == source {
			delete(f.Packages, name)
			dropped = append(dropped, name)
		}
	}

	return dropped
}

// LoadPins applies the recorded sources of installed packages to c, so
// those packages are looked up on their recorded source first. It returns the
// installed packages whose recorded source is no longer configured, mapped to
// that source name.
func (c *Client) LoadPins(file *PinFile, installed func(string) bool) map[string]string {
	missing := map[string]string{}

	c.mu.Lock()
	defer c.mu.Unlock()

	for name, source := range file.Packages {
		if !installed(name) {
			continue
		}

		idx := c.sourceIndex(source)
		if idx < 0 {
			missing[name] = source
			continue
		}

		if _, explicit := c.pins[name]; explicit {
			continue
		}

		c.pins[name] = idx
		c.origin[name] = idx
		c.stored[name] = true
	}

	return missing
}

// RecordInstalled updates the pin file at path for the AUR packages in names
// that were just installed, recording the source each was resolved from, and
// lazily forgets packages that are no longer installed. installed is only
// consulted for packages not in names.
func (c *Client) RecordInstalled(path string, names []string, installed func(string) bool) error {
	if len(c.sources) < 2 || path == "" {
		return nil
	}

	file, err := LoadPinFile(path)
	if err != nil {
		return err
	}

	changed := false
	just := make(map[string]bool, len(names))

	for _, name := range names {
		just[name] = true

		source := ""
		if src, idx := c.sourceOf(name); idx > 0 {
			source = src.Name
		}

		if file.Record(name, source) {
			changed = true
		}
	}

	if len(file.Prune(func(name string) bool { return just[name] || installed(name) })) > 0 {
		changed = true
	}

	if !changed {
		return nil
	}

	return file.Save(path)
}
