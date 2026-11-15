package settings

import (
	"encoding/json"
	"net/url"
	"strings"
)

const defaultAURURL = "https://aur.archlinux.org"

// AURSource is one AUR-compatible endpoint (aurweb RPC + git) that yay can
// search and download PKGBUILDs from.
//
// In config.json an entry can be written either as a plain base URL string or
// as an object:
//
//	"aururls": [
//		"https://aur.archlinux.org",
//		{"name": "mirror", "url": "https://aur.example.org", "rpcurl": "https://aur.example.org/rpc"}
//	]
type AURSource struct {
	Name   string `json:"name,omitempty" lua:"name"`
	URL    string `json:"url" lua:"url"`
	RPCURL string `json:"rpcurl,omitempty" lua:"rpcurl"`
}

// UnmarshalText allows a source to be given as a bare base URL.
func (s *AURSource) UnmarshalText(b []byte) error {
	*s = AURSource{URL: strings.TrimSpace(string(b))}
	return nil
}

// UnmarshalJSON accepts either a base URL string or an object.
func (s *AURSource) UnmarshalJSON(b []byte) error {
	var raw string
	if err := json.Unmarshal(b, &raw); err == nil {
		return s.UnmarshalText([]byte(raw))
	}

	type plain AURSource

	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}

	*s = AURSource(p)

	return nil
}

// MarshalJSON writes the short string form when only the URL is set.
func (s AURSource) MarshalJSON() ([]byte, error) {
	if s.Name == "" && s.RPCURL == "" {
		return json.Marshal(s.URL)
	}

	type plain AURSource

	return json.Marshal(plain(s))
}

// normalizeRPCURL derives the /rpc? endpoint yay's RPC client expects from a
// base URL and an optional explicit RPC URL.
func normalizeRPCURL(baseURL, rpcURL string) string {
	if rpcURL == "" {
		return baseURL + "/rpc?"
	}

	if strings.HasSuffix(rpcURL, "?") {
		return rpcURL
	}

	if strings.HasSuffix(rpcURL, "/rpc") {
		return rpcURL + "?"
	}

	return strings.TrimRight(rpcURL, "/") + "/rpc?"
}

func (s AURSource) normalized() AURSource {
	s.URL = strings.TrimRight(strings.TrimSpace(s.URL), "/")
	s.RPCURL = normalizeRPCURL(s.URL, strings.TrimSpace(s.RPCURL))

	if s.Name == "" {
		s.Name = defaultSourceName(s.URL)
	}

	return s
}

func defaultSourceName(baseURL string) string {
	if baseURL == defaultAURURL {
		return "aur"
	}

	if u, err := url.Parse(baseURL); err == nil && u.Host != "" {
		return u.Host
	}

	return baseURL
}

// AURSources returns the ordered list of AUR sources to query. When aururls is
// not configured, the single aururl/aurrpcurl pair is the only source, which
// keeps the pre-existing behaviour. Entries are normalized and duplicate base
// URLs are dropped (first occurrence wins).
func (c *Configuration) AURSources() []AURSource {
	if len(c.AURURLs) == 0 {
		return []AURSource{AURSource{Name: "aur", URL: c.AURURL, RPCURL: c.AURRPCURL}.normalized()}
	}

	sources := make([]AURSource, 0, len(c.AURURLs))
	seen := make(map[string]bool, len(c.AURURLs))

	for _, src := range c.AURURLs {
		src = src.normalized()
		if src.URL == "" || seen[src.URL] {
			continue
		}

		seen[src.URL] = true
		sources = append(sources, src)
	}

	if len(sources) == 0 {
		return []AURSource{AURSource{Name: "aur", URL: c.AURURL, RPCURL: c.AURRPCURL}.normalized()}
	}

	return sources
}

// AURBaseURLs returns the base URLs of AURSources in order.
func (c *Configuration) AURBaseURLs() []string {
	sources := c.AURSources()
	urls := make([]string, 0, len(sources))

	for _, src := range sources {
		urls = append(urls, src.URL)
	}

	return urls
}

// setAURURLsFromArgs replaces the configured source list with the URLs passed
// via one or more --aururls flags. Each flag value may hold a comma separated
// list.
func (c *Configuration) setAURURLsFromArgs(values []string) {
	var sources []AURSource

	for _, value := range values {
		for part := range strings.SplitSeq(value, ",") {
			if part = strings.TrimSpace(part); part != "" {
				sources = append(sources, AURSource{URL: part})
			}
		}
	}

	if len(sources) > 0 {
		c.AURURLs = sources
	}
}
