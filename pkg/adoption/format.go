package adoption

import "github.com/Jguer/yay/v13/pkg/text"

// Mirrors the age-tag convention already used for recently modified packages.
func FormatWarning(name string, r Record) string {
	age := text.FormatDuration(NowFunc().Sub(r.ObservedAt))

	var msg string
	switch r.Kind {
	case KindOrphanModified:
		msg = name + ": orphan modified " + age + " ago (transient maintainership)"
	default:
		msg = name + ": maintainer changed " + maintainerLabel(r.From) +
			" \u2192 " + maintainerLabel(r.To) + " " + age + " ago"
	}
	return text.Bold(text.Red(msg))
}

func FormatTag(r Record) string {
	age := text.FormatDuration(NowFunc().Sub(r.ObservedAt))

	label := "maintainer changed"
	if r.Kind == KindOrphanModified {
		label = "orphan modified"
	}
	return text.Bold(text.Red("(" + label + " " + age + " ago)"))
}

func maintainerLabel(m string) string {
	if m == "" {
		return "(orphan)"
	}
	return m
}
