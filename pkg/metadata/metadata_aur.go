package metadata

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Jguer/aur"
	"github.com/itchyny/gojq"
	"github.com/ohler55/ojg/oj"
)

const (
	cacheValidity = 1 * time.Hour
)

type AURCacheClient struct {
	httpClient    HTTPRequestDoer
	cachePath     string
	DebugLoggerFn func(a ...interface{})

	unmarshalledCache []interface{}
}

type AURQuery struct {
	Needles  []string
	By       aur.By
	Contains bool // if true, search for packages containing the needle, not exact matches
}

// ClientOption allows setting custom parameters during construction.
type ClientOption func(*AURCacheClient) error

func NewAURCache(httpClient HTTPRequestDoer, cachePath string, opts ...ClientOption) (*AURCacheClient, error) {
	return &AURCacheClient{
		httpClient: httpClient,
		cachePath:  cachePath,
	}, nil
}

// needsUpdate checks if cachepath is older than 24 hours.
func (a *AURCacheClient) needsUpdate() (bool, error) {
	// check if cache is older than 24 hours
	info, err := os.Stat(a.cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}

		return false, fmt.Errorf("unable to read cache: %w", err)
	}

	return info.ModTime().Before(time.Now().Add(-cacheValidity)), nil
}

// Get returns a list of packages that provide the given search term.
func (a *AURCacheClient) Get(ctx context.Context, query *AURQuery) ([]*aur.Pkg, error) {
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

func (a *AURCacheClient) cache(ctx context.Context) ([]interface{}, error) {
	if a.unmarshalledCache != nil {
		return a.unmarshalledCache, nil
	}

	update, err := a.needsUpdate()
	if err != nil {
		return nil, err
	}

	if update {
		if a.DebugLoggerFn != nil {
			a.DebugLoggerFn("AUR Cache is out of date, updating")
		}
		cache, makeErr := MakeCache(ctx, a.httpClient, a.cachePath)
		if makeErr != nil {
			return nil, makeErr
		}

		inputStruct, unmarshallErr := oj.Parse(cache)
		if unmarshallErr != nil {
			return nil, fmt.Errorf("aur metadata unable to parse cache: %w", unmarshallErr)
		}

		a.unmarshalledCache = inputStruct.([]interface{})
	} else {
		aurCache, err := ReadCache(a.cachePath)
		if err != nil {
			return nil, err
		}

		inputStruct, err := oj.Parse(aurCache)
		if err != nil {
			return nil, fmt.Errorf("aur metadata unable to parse cache: %w", err)
		}

		a.unmarshalledCache = inputStruct.([]interface{})
	}

	return a.unmarshalledCache, nil
}

func (a *AURCacheClient) gojqGetBatch(ctx context.Context, query *AURQuery) ([]*aur.Pkg, error) {
	pattern := ".[] | select("

	for i, searchTerm := range query.Needles {
		if i != 0 {
			pattern += ","
		}

		bys := toSearchBy(query.By)
		for j, by := range bys {
			if query.Contains {
				pattern += fmt.Sprintf("(.%s // empty | test(%q))", by, searchTerm)
			} else {
				pattern += fmt.Sprintf("(.%s == %q)", by, searchTerm)
			}

			if j != len(bys)-1 {
				pattern += ","
			}
		}
	}

	pattern += ")"

	if a.DebugLoggerFn != nil {
		a.DebugLoggerFn("AUR metadata query", pattern)
	}

	parsed, err := gojq.Parse(pattern)
	if err != nil {
		return nil, fmt.Errorf("unable to parse query: %w", err)
	}

	unmarshalledCache, errCache := a.cache(ctx)
	if errCache != nil {
		return nil, errCache
	}

	final := make([]*aur.Pkg, 0, len(query.Needles))
	iter := parsed.RunWithContext(ctx, unmarshalledCache) // or query.RunWithContext

	for pkgMap, ok := iter.Next(); ok; pkgMap, ok = iter.Next() {
		if err, ok := pkgMap.(error); ok {
			return nil, err
		}

		pkg := new(aur.Pkg)

		bValue, err := gojq.Marshal(pkgMap)
		if err != nil {
			return nil, fmt.Errorf("unable to marshal aur package: %w", err)
		}

		errU := oj.Unmarshal(bValue, pkg)
		if errU != nil {
			return nil, fmt.Errorf("unable to unmarshal aur package: %w", errU)
		}

		final = append(final, pkg)
	}

	if a.DebugLoggerFn != nil {
		a.DebugLoggerFn("AUR metadata query found", len(final))
	}

	return final, nil
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
