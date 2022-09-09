package main

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v11/pkg/multierror"
	"github.com/Jguer/yay/v11/pkg/settings/exe"
	"github.com/Jguer/yay/v11/pkg/stringset"
	"github.com/Jguer/yay/v11/pkg/text"
)

type ErrDownloadSource struct {
	inner   error
	pkgName string
	errOut  string
}

func (e ErrDownloadSource) Error() string {
	return fmt.Sprintln(gotext.Get("error downloading sources: %s", text.Cyan(e.pkgName)),
		"\n\t context:", e.inner.Error(), "\n\t", e.errOut)
}

func (e *ErrDownloadSource) Unwrap() error {
	return e.inner
}

func downloadPKGBUILDSource(ctx context.Context, cmdBuilder exe.ICmdBuilder, dest,
	base string, incompatible stringset.StringSet,
) error {
	dir := filepath.Join(dest, base)
	args := []string{"--verifysource", "-Ccf"}

	if incompatible.Get(base) {
		args = append(args, "--ignorearch")
	}

	err := cmdBuilder.Show(
		cmdBuilder.BuildMakepkgCmd(ctx, dir, args...))
	if err != nil {
		return ErrDownloadSource{inner: err, pkgName: base, errOut: ""}
	}

	return nil
}

func downloadPKGBUILDSourceWorker(ctx context.Context, wg *sync.WaitGroup, dest string,
	cBase <-chan string, valOut chan<- string, errOut chan<- error,
	cmdBuilder exe.ICmdBuilder, incompatible stringset.StringSet,
) {
	for base := range cBase {
		err := downloadPKGBUILDSource(ctx, cmdBuilder, dest, base, incompatible)
		if err != nil {
			errOut <- ErrDownloadSource{inner: err, pkgName: base, errOut: ""}
		} else {
			valOut <- base
		}
	}

	wg.Done()
}

func downloadPKGBUILDSourceFanout(ctx context.Context, cmdBuilder exe.ICmdBuilder, dest string,
	bases []string, incompatible stringset.StringSet, maxConcurrentDownloads int,
) error {
	if len(bases) == 0 {
		return nil // no work to do
	}

	if len(bases) == 1 {
		return downloadPKGBUILDSource(ctx, cmdBuilder, dest, bases[0], incompatible)
	}

	var (
		numOfWorkers    = runtime.NumCPU()
		wg              = &sync.WaitGroup{}
		c               = make(chan string)
		fanInChanValues = make(chan string)
		fanInChanErrors = make(chan error)
	)

	if maxConcurrentDownloads != 0 {
		numOfWorkers = maxConcurrentDownloads
	}

	go func() {
		for _, base := range bases {
			c <- base
		}

		close(c)
	}()

	// Launch Workers
	wg.Add(numOfWorkers)

	for s := 0; s < numOfWorkers; s++ {
		go downloadPKGBUILDSourceWorker(ctx, wg, dest, c,
			fanInChanValues, fanInChanErrors, cmdBuilder, incompatible)
	}

	go func() {
		wg.Wait()
		close(fanInChanValues)
		close(fanInChanErrors)
	}()

	returnErr := multierror.MultiError{}

receiver:
	for {
		select {
		case _, ok := <-fanInChanValues:
			if !ok {
				break receiver
			}
		case err, ok := <-fanInChanErrors:
			if !ok {
				break receiver
			}
			returnErr.Add(err)
		}
	}

	return returnErr.Return()
}
