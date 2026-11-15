// Package aursource queries several AUR-compatible sources as one and
// remembers which source each package was resolved from.
package aursource

import (
	"context"
	"errors"
	"sync"

	"github.com/Jguer/aur"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v13/pkg/settings"
	"github.com/Jguer/yay/v13/pkg/text"
)

// Client implements aur.QueryClient over an ordered list of sources. Results
// from every source are merged; when two sources provide a package with the
// same name the earlier source wins.
type Client struct {
	sources []settings.AURSource
	clients []aur.QueryClient
	logger  *text.Logger

	mu     sync.RWMutex
	origin map[string]int  // package name or package base -> source index
	pins   map[string]int  // package name -> source index to look the package up on first
	stored map[string]bool // pins that come from the recorded install source
	warned map[string]bool
}

// New returns a Client. sources and clients must have the same length and
// order; clients[i] queries sources[i].
func New(sources []settings.AURSource, clients []aur.QueryClient, logger *text.Logger) *Client {
	if len(sources) != len(clients) {
		panic("aursource: sources and clients length mismatch")
	}

	return &Client{
		sources: sources,
		clients: clients,
		logger:  logger,
		origin:  make(map[string]int),
		pins:    make(map[string]int),
		stored:  make(map[string]bool),
		warned:  make(map[string]bool),
	}
}

// Sources returns the configured sources in priority order.
func (c *Client) Sources() []settings.AURSource {
	return c.sources
}

// Get queries every source concurrently and merges the results in source
// order. A failing source is logged and skipped; an error is returned only
// when every source fails.
func (c *Client) Get(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
	if len(c.clients) == 1 {
		return c.clients[0].Get(ctx, query)
	}

	results := make([][]aur.Pkg, len(c.clients))
	errs := make([]error, len(c.clients))

	var wg sync.WaitGroup

	for i, client := range c.clients {
		wg.Add(1)

		go func(i int, client aur.QueryClient) {
			defer wg.Done()

			results[i], errs[i] = client.Get(ctx, query)
		}(i, client)
	}

	wg.Wait()

	var (
		merged []aur.Pkg
		failed int
		seen   = make(map[string]bool)
	)

	// Packages requested from a source (<source>/<name>) or recorded as
	// installed from one come from that source when it has them, regardless
	// of priority.
	c.mu.Lock()
	for i, pkgs := range results {
		if errs[i] != nil {
			continue
		}

		for j := range pkgs {
			if idx, ok := c.pins[pkgs[j].Name]; ok && idx == i && !seen[pkgs[j].Name] {
				seen[pkgs[j].Name] = true
				merged = append(merged, pkgs[j])
				c.origin[pkgs[j].Name] = i

				if pkgs[j].PackageBase != "" {
					c.origin[pkgs[j].PackageBase] = i
				}
			}
		}
	}
	c.mu.Unlock()

	for i, pkgs := range results {
		if errs[i] != nil {
			failed++

			if c.logger != nil {
				c.logger.Warnln("AUR source", c.sources[i].Name+":", errs[i])
			}

			continue
		}

		for j := range pkgs {
			if seen[pkgs[j].Name] {
				continue
			}

			seen[pkgs[j].Name] = true
			merged = append(merged, pkgs[j])

			if !c.pinFallback(&pkgs[j], i) {
				c.record(&pkgs[j], i)
			}
		}
	}

	if failed == len(c.clients) {
		return nil, errors.Join(errs...)
	}

	return merged, nil
}

// pinFallback handles a pinned package that its pinned source did not return
// and is taken from source idx instead. It reports whether pkg was pinned.
func (c *Client) pinFallback(pkg *aur.Pkg, idx int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	pinned, ok := c.pins[pkg.Name]
	if !ok {
		return false
	}

	c.origin[pkg.Name] = idx
	if pkg.PackageBase != "" {
		c.origin[pkg.PackageBase] = idx
	}

	if c.warned[pkg.Name] || c.logger == nil {
		return true
	}

	c.warned[pkg.Name] = true

	if c.stored[pkg.Name] {
		c.logger.Warnln(gotext.Get(
			"%s was installed from AUR source '%s', which no longer has it; using '%s' instead",
			pkg.Name, c.sources[pinned].Name, c.sources[idx].Name))
	} else {
		c.logger.Warnln(gotext.Get(
			"package '%s' was not found in AUR source '%s', using '%s' instead",
			pkg.Name, c.sources[pinned].Name, c.sources[idx].Name))
	}

	return true
}

func (c *Client) record(pkg *aur.Pkg, idx int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, key := range []string{pkg.Name, pkg.PackageBase} {
		if key == "" {
			continue
		}

		if _, pinned := c.pins[key]; pinned {
			continue
		}

		if prev, ok := c.origin[key]; !ok || idx < prev {
			c.origin[key] = idx
		}
	}
}

// PinAURSource makes pkgName resolve from the source labeled source (as in a
// "<source>/<name>" target) when that source provides it. It reports whether
// source names one of the configured sources.
func (c *Client) PinAURSource(pkgName, source string) bool {
	for i := range c.sources {
		if c.sources[i].Name != source {
			continue
		}

		c.mu.Lock()
		c.pins[pkgName] = i
		c.origin[pkgName] = i
		delete(c.stored, pkgName)
		c.mu.Unlock()

		return true
	}

	return false
}

// UnpinAURSource drops any pin of pkgName, so it resolves in priority order
// (as for an explicit "aur/<name>" target).
func (c *Client) UnpinAURSource(pkgName string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.pins, pkgName)
	delete(c.stored, pkgName)
	delete(c.origin, pkgName)
}

func (c *Client) sourceIndex(name string) int {
	for i := range c.sources {
		if c.sources[i].Name == name {
			return i
		}
	}

	return -1
}

func (c *Client) sourceOf(name string) (src settings.AURSource, idx int) {
	c.mu.RLock()
	idx, ok := c.origin[name]
	c.mu.RUnlock()

	if !ok {
		idx = 0
	}

	return c.sources[idx], idx
}

// SourceFor returns the source a package (by name or package base) was
// resolved from by a previous Get. It falls back to the first source.
func (c *Client) SourceFor(name string) settings.AURSource {
	src, _ := c.sourceOf(name)
	return src
}

// URLsFor returns the base URLs to try when downloading name, starting with
// the source it was resolved from and followed by the remaining sources in
// priority order.
func (c *Client) URLsFor(name string) []string {
	first := c.SourceFor(name).URL
	urls := make([]string, 0, len(c.sources))
	urls = append(urls, first)

	for _, src := range c.sources {
		if src.URL != first {
			urls = append(urls, src.URL)
		}
	}

	return urls
}
