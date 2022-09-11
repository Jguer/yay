package main

import (
	"context"
	"fmt"
	"runtime"
	"sync"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v11/pkg/multierror"
	"github.com/Jguer/yay/v11/pkg/settings/exe"
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

func downloadPKGBUILDSource(ctx context.Context,
	cmdBuilder exe.ICmdBuilder, pkgBuildDir string, installIncompatible bool,
) error {
	args := []string{"--verifysource", "-Ccf"}

	if installIncompatible {
		args = append(args, "--ignorearch")
	}

	err := cmdBuilder.Show(
		cmdBuilder.BuildMakepkgCmd(ctx, pkgBuildDir, args...))
	if err != nil {
		return ErrDownloadSource{inner: err}
	}

	return nil
}

func downloadPKGBUILDSourceWorker(ctx context.Context, wg *sync.WaitGroup,
	dirChannel <-chan string, valOut chan<- string, errOut chan<- error,
	cmdBuilder exe.ICmdBuilder, incompatible bool,
) {
	for pkgBuildDir := range dirChannel {
		err := downloadPKGBUILDSource(ctx, cmdBuilder, pkgBuildDir, incompatible)
		if err != nil {
			errOut <- ErrDownloadSource{inner: err, pkgName: pkgBuildDir, errOut: ""}
		} else {
			valOut <- pkgBuildDir
		}
	}

	wg.Done()
}

func downloadPKGBUILDSourceFanout(ctx context.Context, cmdBuilder exe.ICmdBuilder, pkgBuildDirs []string,
	incompatible bool, maxConcurrentDownloads int,
) error {
	if len(pkgBuildDirs) == 0 {
		return nil // no work to do
	}

	if len(pkgBuildDirs) == 1 {
		return downloadPKGBUILDSource(ctx, cmdBuilder, pkgBuildDirs[0], incompatible)
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
		for _, pkgbuildDir := range pkgBuildDirs {
			c <- pkgbuildDir
		}

		close(c)
	}()

	// Launch Workers
	wg.Add(numOfWorkers)

	for s := 0; s < numOfWorkers; s++ {
		go downloadPKGBUILDSourceWorker(ctx, wg, c,
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
