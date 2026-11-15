// Package aursource queries several AUR-compatible sources as one and
// remembers which source each package was resolved from.
package aursource

import (
	"context"
	"errors"
	"sync"

	"github.com/Jguer/aur"

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
	origin map[string]int // package name or package base -> source index
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
			c.record(&pkgs[j], i)
		}
	}

	if failed == len(c.clients) {
		return nil, errors.Join(errs...)
	}

	return merged, nil
}

func (c *Client) record(pkg *aur.Pkg, idx int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, key := range []string{pkg.Name, pkg.PackageBase} {
		if key == "" {
			continue
		}

		if prev, ok := c.origin[key]; !ok || idx < prev {
			c.origin[key] = idx
		}
	}
}

// SourceFor returns the source a package (by name or package base) was
// resolved from by a previous Get. It falls back to the first source.
func (c *Client) SourceFor(name string) settings.AURSource {
	c.mu.RLock()
	idx, ok := c.origin[name]
	c.mu.RUnlock()

	if !ok {
		idx = 0
	}

	return c.sources[idx]
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
