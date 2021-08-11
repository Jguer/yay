package query

import (
	"context"
	"sync"

	"github.com/Jguer/aur"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/intrange"
	"github.com/Jguer/yay/v10/pkg/multierror"
	"github.com/Jguer/yay/v10/pkg/text"
)

type Pkg = aur.Pkg

// Queries the aur for information about specified packages.
// All packages should be queried in a single aur request except when the number
// of packages exceeds the number set in config.RequestSplitN.
// If the number does exceed config.RequestSplitN multiple aur requests will be
// performed concurrently.
func AURInfo(aurClient *aur.Client, names []string, warnings *AURWarnings, splitN int) ([]*Pkg, error) {
	info := make([]*Pkg, 0, len(names))
	seen := make(map[string]int)

	var (
		mux  sync.Mutex
		wg   sync.WaitGroup
		errs multierror.MultiError
	)

	makeRequest := func(n, max int) {
		defer wg.Done()

		tempInfo, requestErr := aurClient.Info(context.Background(), names[n:max])
		if requestErr != nil {
			errs.Add(requestErr)
			return
		}

		mux.Lock()
		for i := range tempInfo {
			info = append(info, &tempInfo[i])
		}
		mux.Unlock()
	}

	for n := 0; n < len(names); n += splitN {
		max := intrange.Min(len(names), n+splitN)

		wg.Add(1)

		go makeRequest(n, max)
	}

	wg.Wait()

	if err := errs.Return(); err != nil {
		return info, err
	}

	for k, pkg := range info {
		seen[pkg.Name] = k
	}

	for _, name := range names {
		i, ok := seen[name]
		if !ok && !warnings.Ignore.Get(name) {
			warnings.Missing = append(warnings.Missing, name)
			continue
		}

		pkg := info[i]

		if pkg.Maintainer == "" && !warnings.Ignore.Get(name) {
			warnings.Orphans = append(warnings.Orphans, name)
		}

		if pkg.OutOfDate != 0 && !warnings.Ignore.Get(name) {
			warnings.OutOfDate = append(warnings.OutOfDate, name)
		}
	}

	return info, nil
}

func AURInfoPrint(aurClient *aur.Client, names []string, splitN int) ([]*Pkg, error) {
	text.OperationInfoln(gotext.Get("Querying AUR..."))

	warnings := &AURWarnings{}

	info, err := AURInfo(aurClient, names, warnings, splitN)
	if err != nil {
		return info, err
	}

	warnings.Print()

	return info, nil
}
