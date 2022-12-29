package main

import (
	"context"

	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/pgp"
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/settings/exe"

	gosrc "github.com/Morganamilo/go-srcinfo"
)

type srcinfoOperator struct {
	dbExecutor db.Executor
	cfg        *settings.Configuration
	cmdBuilder exe.ICmdBuilder
}

func (s *srcinfoOperator) Run(ctx context.Context, pkgbuildDirs map[string]string) (map[string]*gosrc.Srcinfo, error) {
	srcinfos, err := parseSrcinfoFiles(pkgbuildDirs, true)
	if err != nil {
		return nil, err
	}

	if err := confirmIncompatibleInstall(srcinfos, s.dbExecutor); err != nil {
		return nil, err
	}

	if s.cfg.PGPFetch {
		if _, errCPK := pgp.CheckPgpKeys(ctx, pkgbuildDirs, srcinfos, s.cmdBuilder, settings.NoConfirm); errCPK != nil {
			return nil, errCPK
		}
	}

	return srcinfos, nil
}
