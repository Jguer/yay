package srcinfo

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	gosrc "github.com/Morganamilo/go-srcinfo"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/dep"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/exe"
	"github.com/Jguer/yay/v12/pkg/sync/srcinfo/pgp"
	"github.com/Jguer/yay/v12/pkg/text"
	"github.com/Jguer/yay/v12/pkg/vcs"
)

type Service struct {
	dbExecutor db.Executor
	cfg        *settings.Configuration
	cmdBuilder pgp.GPGCmdBuilder
	vcsStore   vcs.Store
	log        *text.Logger

	pkgBuildDirs map[string]string
	srcInfos     map[string]*gosrc.Srcinfo
}

func NewService(dbExecutor db.Executor, cfg *settings.Configuration, logger *text.Logger,
	cmdBuilder exe.ICmdBuilder, vcsStore vcs.Store, pkgBuildDirs map[string]string,
) (*Service, error) {
	srcinfos, err := ParseSrcinfoFilesByBase(logger, pkgBuildDirs, true)
	if err != nil {
		panic(err)
	}
	return &Service{
		dbExecutor:   dbExecutor,
		cfg:          cfg,
		cmdBuilder:   cmdBuilder,
		vcsStore:     vcsStore,
		pkgBuildDirs: pkgBuildDirs,
		srcInfos:     srcinfos,
		log:          logger,
	}, nil
}

func (s *Service) IncompatiblePkgs(ctx context.Context) ([]string, error) {
	incompatible := []string{}

	alpmArch, err := s.dbExecutor.AlpmArchitectures()
	if err != nil {
		return nil, err
	}

nextpkg:
	for base, srcinfo := range s.srcInfos {
		for _, arch := range srcinfo.Arch {
			if db.ArchIsSupported(alpmArch, arch) {
				continue nextpkg
			}
		}
		incompatible = append(incompatible, base)
	}

	return incompatible, nil
}

func (s *Service) CheckPGPKeys(ctx context.Context) error {
	_, errCPK := pgp.CheckPgpKeys(ctx, s.log.Child("pgp"), s.pkgBuildDirs, s.srcInfos, s.cmdBuilder, settings.NoConfirm)
	return errCPK
}

func (s *Service) UpdateVCSStore(ctx context.Context, targets []map[string]*dep.InstallInfo, ignore map[string]error,
) error {
	for _, srcinfo := range s.srcInfos {
		if srcinfo.Source == nil {
			continue
		}

		// TODO: high complexity - refactor
		for i := range srcinfo.Packages {
			for j := range targets {
				if _, ok := targets[j][srcinfo.Packages[i].Pkgname]; !ok {
					s.log.Debugln("skipping VCS update for", srcinfo.Packages[i].Pkgname, "not in targets")
					continue
				}
				if _, ok := ignore[srcinfo.Packages[i].Pkgname]; ok {
					s.log.Debugln("skipping VCS update for", srcinfo.Packages[i].Pkgname, "due to install error")
					continue
				}

				s.log.Debugln("checking VCS entry for", srcinfo.Packages[i].Pkgname, fmt.Sprintf("source: %v", srcinfo.Source))
				s.vcsStore.Update(ctx, srcinfo.Packages[i].Pkgname, srcinfo.Source)
			}
		}
	}

	return nil
}

func ParseSrcinfoFilesByBase(logger *text.Logger, pkgBuildDirs map[string]string, errIsFatal bool) (map[string]*gosrc.Srcinfo, error) {
	srcinfos := make(map[string]*gosrc.Srcinfo)

	k := 0
	for base, dir := range pkgBuildDirs {
		logger.OperationInfoln(gotext.Get("(%d/%d) Parsing SRCINFO: %s", k+1, len(pkgBuildDirs), text.Cyan(base)))

		pkgbuild, err := gosrc.ParseFile(filepath.Join(dir, ".SRCINFO"))
		if err != nil {
			if !errIsFatal {
				logger.Warnln(gotext.Get("failed to parse %s -- skipping: %s", base, err))
				continue
			}

			return nil, errors.New(gotext.Get("failed to parse %s: %s", base, err))
		}

		srcinfos[base] = pkgbuild
		k++
	}

	return srcinfos, nil
}
