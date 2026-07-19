package srcinfo

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	gosrc "github.com/Morganamilo/go-srcinfo"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v13/pkg/db"
	"github.com/Jguer/yay/v13/pkg/dep"
	"github.com/Jguer/yay/v13/pkg/settings"
	"github.com/Jguer/yay/v13/pkg/settings/exe"
	"github.com/Jguer/yay/v13/pkg/sync/srcinfo/pgp"
	"github.com/Jguer/yay/v13/pkg/text"
	"github.com/Jguer/yay/v13/pkg/vcs"
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
		return nil, err
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
	targetPackages := make(map[string]struct{})
	for _, target := range targets {
		for pkgName := range target {
			targetPackages[pkgName] = struct{}{}
		}
	}

	for _, srcinfo := range s.srcInfos {
		if srcinfo.Source == nil {
			continue
		}

		for i := range srcinfo.Packages {
			pkgName := srcinfo.Packages[i].Pkgname
			if _, ok := targetPackages[pkgName]; !ok {
				s.log.Debugln("skipping VCS update for", pkgName, "not in targets")
				continue
			}
			if _, ok := ignore[pkgName]; ok {
				s.log.Debugln("skipping VCS update for", pkgName, "due to install error")
				continue
			}

			s.log.Debugln("checking VCS entry for", pkgName, fmt.Sprintf("source: %v", srcinfo.Source))
			s.vcsStore.Update(ctx, pkgName, srcinfo.Source)
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
