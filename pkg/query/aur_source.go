package query

import (
	"github.com/Jguer/yay/v13/pkg/settings"
	"github.com/Jguer/yay/v13/pkg/text"
)

// DefaultAURLabel is the repository label shown for packages from the default
// AUR source. Internally every AUR-type package keeps the "aur" source marker;
// the label only affects what is displayed.
const DefaultAURLabel = "aur"

// AURSourceResolver is implemented by AUR clients that query several sources
// and remember which source each package was resolved from.
type AURSourceResolver interface {
	SourceFor(name string) settings.AURSource
}

// AURSourceOf returns the display label and base URL of the AUR source pkgName
// was resolved from. Clients without source information report the default
// AUR label and an empty URL.
func AURSourceOf(client any, pkgName string) (label, baseURL string) {
	resolver, ok := client.(AURSourceResolver)
	if !ok {
		return DefaultAURLabel, ""
	}

	src := resolver.SourceFor(pkgName)
	if src.Name == "" {
		return DefaultAURLabel, src.URL
	}

	return src.Name, src.URL
}

// AURSourceLabel returns only the display label, see AURSourceOf.
func AURSourceLabel(client any, pkgName string) string {
	label, _ := AURSourceOf(client, pkgName)
	return label
}

// aurRepoLink renders "<label>/<name>" with an OSC 8 link to the package page.
// The default label keeps the historical aur.archlinux.org link.
func aurRepoLink(label, baseURL, pkgName string) string {
	linkText := text.Bold(text.ColorHash(label)) + "/" + text.Bold(pkgName)

	if label == DefaultAURLabel || baseURL == "" {
		return text.CreateRepoLink(DefaultAURLabel, "", pkgName, linkText)
	}

	if !text.UseColor {
		return linkText
	}

	return text.CreateOSC8Link(baseURL+"/packages/"+pkgName, linkText)
}
