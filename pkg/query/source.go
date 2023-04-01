package query

import (
	"context"

	"github.com/Jguer/aur"
	"github.com/hashicorp/go-multierror"
)

type SearchVerbosity int

// Verbosity settings for search.
const (
	NumberMenu SearchVerbosity = iota
	Detailed
	Minimal
)

// queryAUR searches AUR and narrows based on subarguments.
func queryAUR(ctx context.Context,
	aurClient aur.QueryClient,
	pkgS []string, searchBy string,
) ([]aur.Pkg, error) {
	var (
		err error
		by  = getSearchBy(searchBy)
	)

	for _, word := range pkgS {
		r, errM := aurClient.Get(ctx, &aur.Query{
			Needles:  []string{word},
			By:       by,
			Contains: true,
		})
		if errM == nil {
			return r, nil
		}

		err = multierror.Append(err, errM)
	}

	return nil, err
}
