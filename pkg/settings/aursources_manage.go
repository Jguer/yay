package settings

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/leonelquinteros/gotext"
)

// EffectiveName is the name used to reference the source: its explicit name,
// or a name derived from its URL.
func (s AURSource) EffectiveName() string {
	if s.Name != "" {
		return s.Name
	}

	return defaultSourceName(strings.TrimRight(strings.TrimSpace(s.URL), "/"))
}

var reservedSourceNames = []string{
	"core", "extra", "multilib", "testing", "core-testing", "extra-testing",
	"multilib-testing", "gnome-unstable", "kde-unstable", "local", "devel",
}

func validateSourceName(name string) error {
	if name == "" {
		return errors.New(gotext.Get("source name must not be empty"))
	}

	if strings.ContainsAny(name, " \t\n,=/") {
		return errors.New(gotext.Get("invalid source name '%s'", name))
	}

	// Source names double as "<source>/<pkg>" prefixes and repository labels,
	// so they must not shadow official repositories or yay's own labels.
	if slices.Contains(reservedSourceNames, name) {
		return errors.New(gotext.Get("source name '%s' is reserved", name))
	}

	return nil
}

func validateSourceURL(raw string) error {
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		return errors.New(gotext.Get("invalid source URL '%s': must start with http:// or https://", raw))
	}

	return nil
}

// editableAURURLs returns the stored source list, seeding it from
// aururl/aurrpcurl when no list has been configured yet so that adding a
// source keeps the current default as the first entry.
func (c *Configuration) editableAURURLs() []AURSource {
	if len(c.AURURLs) > 0 {
		return c.AURURLs
	}

	return []AURSource{{Name: "aur", URL: strings.TrimRight(c.AURURL, "/"), RPCURL: c.AURRPCURL}}
}

func findSource(sources []AURSource, name string) int {
	for i := range sources {
		if sources[i].EffectiveName() == name {
			return i
		}
	}

	return -1
}

// AddAURSource appends a named source to the list.
func (c *Configuration) AddAURSource(name, rawURL, rpcURL string) error {
	if err := validateSourceName(name); err != nil {
		return err
	}

	if err := validateSourceURL(rawURL); err != nil {
		return err
	}

	sources := c.editableAURURLs()
	if findSource(sources, name) >= 0 {
		return errors.New(gotext.Get("AUR source '%s' already exists", name))
	}

	sources = append(sources, AURSource{Name: name, URL: strings.TrimRight(rawURL, "/"), RPCURL: rpcURL})
	c.AURURLs = sources

	return nil
}

// EditAURSource updates a source. Only keys present in changes are modified;
// accepted keys are "name", "url" and "rpcurl" (an empty rpcurl resets it to
// the derived default).
func (c *Configuration) EditAURSource(name string, changes map[string]string) error {
	sources := c.editableAURURLs()

	idx := findSource(sources, name)
	if idx < 0 {
		return errors.New(gotext.Get("AUR source '%s' not found", name))
	}

	if len(changes) == 0 {
		return errors.New(gotext.Get("nothing to change for AUR source '%s'", name))
	}

	src := sources[idx]
	src.Name = src.EffectiveName()

	for key, value := range changes {
		switch key {
		case "name":
			if err := validateSourceName(value); err != nil {
				return err
			}

			if other := findSource(sources, value); other >= 0 && other != idx {
				return errors.New(gotext.Get("AUR source '%s' already exists", value))
			}

			src.Name = value
		case "url":
			if err := validateSourceURL(value); err != nil {
				return err
			}

			src.URL = strings.TrimRight(value, "/")
		case "rpcurl":
			src.RPCURL = value
		default:
			return errors.New(gotext.Get("unknown AUR source field '%s' (expected name, url or rpcurl)", key))
		}
	}

	sources = append([]AURSource(nil), sources...)
	sources[idx] = src
	c.AURURLs = sources

	return nil
}

// DeleteAURSource removes a source. Removing the last source empties the list,
// which falls back to aururl.
func (c *Configuration) DeleteAURSource(name string) error {
	sources := c.editableAURURLs()

	idx := findSource(sources, name)
	if idx < 0 {
		return errors.New(gotext.Get("AUR source '%s' not found", name))
	}

	c.AURURLs = append(append([]AURSource(nil), sources[:idx]...), sources[idx+1:]...)

	return nil
}

// MoveAURSource moves a source to a 1-based position in the priority order.
func (c *Configuration) MoveAURSource(name string, position int) error {
	sources := append([]AURSource(nil), c.editableAURURLs()...)

	idx := findSource(sources, name)
	if idx < 0 {
		return errors.New(gotext.Get("AUR source '%s' not found", name))
	}

	if position < 1 || position > len(sources) {
		return fmt.Errorf("%s", gotext.Get("position must be between 1 and %d", len(sources)))
	}

	src := sources[idx]
	sources = append(sources[:idx], sources[idx+1:]...)
	sources = append(sources[:position-1], append([]AURSource{src}, sources[position-1:]...)...)
	c.AURURLs = sources

	return nil
}

// UpdateConfigFile loads the JSON config at configPath (without command line
// or init.lua overrides), applies fn and writes it back.
func UpdateConfigFile(configPath, version string, fn func(*Configuration) error) (*Configuration, error) {
	cfg := DefaultConfig(version)
	if err := cfg.loadStrict(configPath); err != nil {
		return nil, err
	}

	if err := fn(cfg); err != nil {
		return nil, err
	}

	if err := cfg.Save(configPath, version); err != nil {
		return nil, err
	}

	return cfg, nil
}
