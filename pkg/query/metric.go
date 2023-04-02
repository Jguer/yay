package query

import (
	"hash/fnv"
	"strings"

	"github.com/adrg/strutil"
)

const minVotes = 30

// TODO: Add support for Popularity and LastModified
func (a *abstractResults) aurSortByMetric(pkg *abstractResult) float64 {
	return 1 - (minVotes / (minVotes + float64(pkg.votes)))
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
	if pkg.source == sourceAUR {
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

	switch source {
	case sourceAUR:
		return 0
	case "core":
		return 40
	case "extra":
		return 30
	case "community":
		return 20
	case "multilib":
		return 10
	}

	if v, ok := a.separateSourceCache[source]; ok {
		return v
	}

	h := fnv.New32a()
	h.Write([]byte(source))
	sourceScore := float64(int(h.Sum32())%9 + 2)
	a.separateSourceCache[source] = sourceScore

	return sourceScore
}

func (a *abstractResults) calculateMetric(pkg *abstractResult) float64 {
	score := a.GetMetric(pkg)
	return a.separateSourceScore(pkg.source, score) + score
}
