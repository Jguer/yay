package metadata

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/Jguer/aur"
	"github.com/itchyny/gojq"
	"github.com/ohler55/ojg/oj"
	"github.com/tidwall/gjson"
)

type AURCache struct {
	cache             []byte
	provideCache      map[string][]*aur.Pkg
	unmarshalledCache []interface{}
	cacheHits         int
	gojqCode          *gojq.Code
}

func NewAURCache(cachePath string) (*AURCache, error) {
	aurCache, err := MakeOrReadCache(cachePath)
	if err != nil {
		return nil, err
	}
	inputStruct, err := oj.Parse(aurCache)

	return &AURCache{
		cache:             aurCache,
		provideCache:      make(map[string][]*aur.Pkg, 300),
		unmarshalledCache: inputStruct.([]interface{}),
		gojqCode:          makeGoJQ(),
	}, nil
}

func (a *AURCache) DebugInfo() {
	fmt.Println("Byte Cache", len(a.cache))
	fmt.Println("Entries Cached", len(a.provideCache))
	fmt.Println("Cache Hits", a.cacheHits)
}

func (a *AURCache) SetProvideCache(needle string, pkgs []*aur.Pkg) {
	a.provideCache[needle] = pkgs
}

// Get returns a list of packages that provide the given search term
func (a *AURCache) FindPackage(needle string) ([]*aur.Pkg, error) {
	if pkgs, ok := a.provideCache[needle]; ok {
		a.cacheHits++
		return pkgs, nil
	}

	final, error := a.gojqGet(needle)
	if error != nil {
		return nil, error
	}

	a.provideCache[needle] = final

	return final, nil
}

func (a *AURCache) gjsonGet(depName string) ([]*aur.Pkg, error) {
	dedupMap := make(map[string]bool)
	queryProvides := fmt.Sprintf("#(Provides.#(==\"%s\"))#", depName)
	queryNames := fmt.Sprintf("#(Name==\"%s\")#", depName)
	queryBases := fmt.Sprintf("#(PackageBase==\"%s\")#", depName)

	results := gjson.GetManyBytes(a.cache, queryProvides, queryNames, queryBases)

	aggregated := append(append(results[0].Array(), results[1].Array()...), results[2].Array()...)

	final := make([]*aur.Pkg, 0, len(aggregated))

	for i := range aggregated {
		jsonString := aggregated[i].Raw
		key := jsonString[:15]

		if _, ok := dedupMap[key]; !ok {
			pkg := &aur.Pkg{}
			json.Unmarshal([]byte(jsonString), pkg)
			final = append(final, pkg)
			dedupMap[key] = true
		}
	}
	return final, nil
}

func (a *AURCache) gojqGet(searchTerm string) ([]*aur.Pkg, error) {
	final := make([]*aur.Pkg, 0, 1)

	iter := a.gojqCode.Run(a.unmarshalledCache, searchTerm) // or query.RunWithContext
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
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
	pattern := ".[] | select((.PackageBase == $x) or (.Name == $x) or (.Provides[]? == ($x)))"
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
