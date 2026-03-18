package query

import (
	"strings"

	"github.com/adrg/strutil"
)

const minVotes = 30
const minPopularity = 0.5
const (
	separateSourceMax = 45.0
	separateSourceMin = 5.0
)

func (a *abstractResults) aurSortByMetric(pkg *abstractResult) float64 {
	votesScore := 1 - (minVotes / (minVotes + float64(pkg.votes)))
	if pkg.popularity <= 0 {
		return votesScore
	}

	popularityScore := 1 - (minPopularity / (minPopularity + pkg.popularity))

	return (votesScore + popularityScore) / 2
}

func (a *abstractResults) GetMetric(pkg *abstractResult) float64 {
	if v, ok := a.distanceCache[pkg.name]; ok {
		return v
	}

	if strings.EqualFold(pkg.name, a.search) {
		return 1.0
	}

	sim := strutil.Similarity(pkg.name, a.search, a.metric)

	for _, prov := range pkg.provides {
		// If the package provides search, it's a perfect match
		// AUR packages don't populate provides
		candidate := strutil.Similarity(prov, a.search, a.metric) * 0.80
		if candidate > sim {
			sim = candidate
		}
	}

	simDesc := strutil.Similarity(pkg.description, a.search, a.metric)

	// slightly overweight sync sources by always giving them max popularity
	popularity := 1.0
	if pkg.source == "aur" {
		popularity = a.aurSortByMetric(pkg)
	}

	sim = sim*0.5 + simDesc*0.2 + popularity*0.3

	a.distanceCache[pkg.name] = sim

	return sim
}

func (a *abstractResults) separateSourceScore(source string, score float64) float64 {
	if !a.separateSources {
		return 0
	}

	if score == 1.0 {
		return 50
	}

	if v, ok := a.separateSourceCache[source]; ok {
		return v
	}

	// AUR is always lowest priority
	if source == "aur" {
		return 0
	}

	// Score sync repositories based on pacman.conf order (as reflected by dbExecutor.Repos()).
	// First repo gets max, last repo gets min, evenly distributed across the range.
	for i, repo := range a.repoOrder {
		if repo != source {
			continue
		}

		n := len(a.repoOrder)
		if n == 1 {
			a.separateSourceCache[source] = separateSourceMax
			return separateSourceMax
		}

		step := (separateSourceMax - separateSourceMin) / float64(n-1)
		sourceScore := separateSourceMax - (float64(i) * step)
		a.separateSourceCache[source] = sourceScore
		return sourceScore
	}

	return 0
}

func (a *abstractResults) calculateMetric(pkg *abstractResult) float64 {
	score := a.GetMetric(pkg)
	return a.separateSourceScore(pkg.source, score) + score
}
