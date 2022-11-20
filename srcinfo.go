package main

import (
	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/pgp"
	"github.com/Jguer/yay/v11/pkg/settings"

	gosrc "github.com/Morganamilo/go-srcinfo"
)

type srcinfoOperator struct {
	dbExecutor db.Executor
}

func (s *srcinfoOperator) Run(pkgbuildDirs map[string]string) (map[string]*gosrc.Srcinfo, error) {
	srcinfos, err := parseSrcinfoFiles(pkgbuildDirs, true)
	if err != nil {
		return nil, err
	}

	if err := confirmIncompatibleInstall(srcinfos, s.dbExecutor); err != nil {
		return nil, err
	}

	if config.PGPFetch {
		if _, errCPK := pgp.CheckPgpKeys(pkgbuildDirs, srcinfos, config.GpgBin, config.GpgFlags, settings.NoConfirm); errCPK != nil {
			return nil, errCPK
		}
	}

	return srcinfos, nil
}
