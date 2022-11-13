package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Jguer/aur"
	"github.com/itchyny/gojq"
	"github.com/ohler55/ojg/oj"
)

const (
	searchCacheCap = 300
	cacheValidity  = 1 * time.Hour
)

type AURCache struct {
	cache             []byte
	searchCache       map[string][]*aur.Pkg
	cachePath         string
	unmarshalledCache []interface{}
	cacheHits         int
	gojqCode          *gojq.Code
	DebugLoggerFn     func(a ...interface{})
}

type AURQuery struct {
	Needles []string
	By      aur.By
}

func NewAURCache(cachePath string) (*AURCache, error) {
	aurCache, err := MakeOrReadCache(cachePath)
	if err != nil {
		return nil, err
	}
	inputStruct, err := oj.Parse(aurCache)

	return &AURCache{
		cache:             aurCache,
		cachePath:         cachePath,
		searchCache:       make(map[string][]*aur.Pkg, searchCacheCap),
		unmarshalledCache: inputStruct.([]interface{}),
		gojqCode:          makeGoJQ(),
	}, nil
}

// needsUpdate checks if cachepath is older than 24 hours
func (a *AURCache) needsUpdate() (bool, error) {
	// check if cache is older than 24 hours
	info, err := os.Stat(a.cachePath)
	if err != nil {
		return false, fmt.Errorf("unable to read cache: %w", err)
	}

	return info.ModTime().Before(time.Now().Add(-cacheValidity)), nil
}

func (a *AURCache) cacheKey(needle string, byProvides, byBase, byName bool) string {
	return fmt.Sprintf("%s-%v-%v-%v", needle, byProvides, byBase, byName)
}

func (a *AURCache) DebugInfo() {
	fmt.Println("Byte Cache", len(a.cache))
	fmt.Println("Entries Cached", len(a.searchCache))
	fmt.Println("Cache Hits", a.cacheHits)
}

func (a *AURCache) SetProvideCache(needle string, pkgs []*aur.Pkg) {
	a.searchCache[needle] = pkgs
}

// Get returns a list of packages that provide the given search term.
func (a *AURCache) Get(ctx context.Context, query *AURQuery) ([]*aur.Pkg, error) {
	update, err := a.needsUpdate()
	if err != nil {
		return nil, err
	}

	if update {
		if a.DebugLoggerFn != nil {
			a.DebugLoggerFn("AUR Cache is out of date, updating")
		}

		var makeErr error
		if a.cache, makeErr = MakeCache(a.cachePath); makeErr != nil {
			return nil, makeErr
		}

		inputStruct, unmarshallErr := oj.Parse(a.cache)
		if unmarshallErr != nil {
			return nil, unmarshallErr
		}

		a.unmarshalledCache = inputStruct.([]interface{})
	}

	found := make([]*aur.Pkg, 0, len(query.Needles))
	if len(query.Needles) == 0 {
		return found, nil
	}

	iterFound, errNeedle := a.gojqGetBatch(ctx, query)
	if errNeedle != nil {
		return nil, errNeedle
	}

	found = append(found, iterFound...)

	return found, nil
}

// Get returns a list of packages that provide the given search term
func (a *AURCache) FindPackage(ctx context.Context, needle string) ([]*aur.Pkg, error) {
	cacheKey := a.cacheKey(needle, true, true, true)
	if pkgs, ok := a.searchCache[cacheKey]; ok {
		a.cacheHits++
		return pkgs, nil
	}

	final, error := a.gojqGet(ctx, needle)
	if error != nil {
		return nil, error
	}

	a.searchCache[cacheKey] = final

	return final, nil
}

func (a *AURCache) gojqGetBatch(ctx context.Context, query *AURQuery) ([]*aur.Pkg, error) {
	pattern := ".[] | select("

	for i, searchTerm := range query.Needles {
		if i != 0 {
			pattern += " or "
		}

		bys := toSearchBy(query.By)
		for j, by := range bys {
			pattern += fmt.Sprintf("(.%s == \"%s\")", by, searchTerm)
			if j != len(bys)-1 {
				pattern += " or "
			}
		}
	}

	pattern += ")"

	parsed, err := gojq.Parse(pattern)
	if err != nil {
		log.Fatalln(err)
	}

	final := make([]*aur.Pkg, 0, len(query.Needles))

	iter := parsed.RunWithContext(ctx, a.unmarshalledCache) // or query.RunWithContext

	for v, ok := iter.Next(); ok; v, ok = iter.Next() {
		if err, ok := v.(error); ok {
			return nil, err
		}

		pkg := new(aur.Pkg)
		bValue, err := gojq.Marshal(v)
		if err != nil {
			log.Fatalln(err)
		}

		oj.Unmarshal(bValue, pkg)
		final = append(final, pkg)
	}

	if a.DebugLoggerFn != nil {
		a.DebugLoggerFn("AUR Query", pattern, "Found", len(final))
	}

	return final, nil
}

func (a *AURCache) gojqGet(ctx context.Context, searchTerm string) ([]*aur.Pkg, error) {
	final := make([]*aur.Pkg, 0, 1)

	iter := a.gojqCode.RunWithContext(ctx, a.unmarshalledCache, searchTerm) // or query.RunWithContext

	for v, ok := iter.Next(); ok; v, ok = iter.Next() {
		if err, ok := v.(error); ok {
			return nil, err
		}

		pkg := &aur.Pkg{}
		bValue, err := gojq.Marshal(v)
		if err != nil {
			log.Fatalln(err)
		}

		json.Unmarshal(bValue, pkg)
		final = append(final, pkg)
	}

	return final, nil
}

func makeGoJQ() *gojq.Code {
	pattern := ".[] | select((.Name == $x) or (.Provides[]? == ($x)))"

	query, err := gojq.Parse(pattern)
	if err != nil {
		log.Fatalln(err)
	}

	compiled, err := gojq.Compile(query, gojq.WithVariables([]string{"$x"}))
	if err != nil {
		log.Fatalln(err)
	}

	return compiled
}

func toSearchBy(by aur.By) []string {
	switch by {
	case aur.Name:
		return []string{"Name"}
	case aur.NameDesc:
		return []string{"Name", "Description"}
	case aur.Maintainer:
		return []string{"Maintainer"}
	case aur.Depends:
		return []string{"Depends[]?"}
	case aur.MakeDepends:
		return []string{"MakeDepends[]?"}
	case aur.OptDepends:
		return []string{"OptDepends[]?"}
	case aur.CheckDepends:
		return []string{"CheckDepends[]?"}
	case aur.None:
		return []string{"Name", "Provides[]?"}
	default:
		panic("invalid By")
	}
}
