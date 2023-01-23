package srcinfo

import (
	"context"
	"errors"
	"path/filepath"

	gosrc "github.com/Morganamilo/go-srcinfo"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/pgp"
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/settings/exe"
	"github.com/Jguer/yay/v11/pkg/text"
	"github.com/Jguer/yay/v11/pkg/vcs"
)

type Service struct {
	dbExecutor db.Executor
	cfg        *settings.Configuration
	cmdBuilder exe.ICmdBuilder
	vcsStore   vcs.Store

	pkgBuildDirs map[string]string
	srcInfos     map[string]*gosrc.Srcinfo
}

func NewService(dbExecutor db.Executor, cfg *settings.Configuration,
	cmdBuilder exe.ICmdBuilder, vcsStore vcs.Store, pkgBuildDirs map[string]string,
) (*Service, error) {
	srcinfos, err := ParseSrcinfoFiles(pkgBuildDirs, true)
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
	_, errCPK := pgp.CheckPgpKeys(ctx, s.pkgBuildDirs, s.srcInfos, s.cmdBuilder, settings.NoConfirm)
	return errCPK
}

func (s *Service) UpdateVCSStore(ctx context.Context,
	srcinfos map[string]*gosrc.Srcinfo, ignore map[string]any,
) error {
	for _, srcinfo := range srcinfos {
		if srcinfo.Source == nil {
			continue
		}

		if _, ok := ignore[srcinfo.Pkgname]; ok {
			continue
		}

		s.vcsStore.Update(ctx, srcinfo.Pkgname, srcinfo.Source)
	}

	return nil
}

func ParseSrcinfoFiles(pkgBuildDirs map[string]string, errIsFatal bool) (map[string]*gosrc.Srcinfo, error) {
	srcinfos := make(map[string]*gosrc.Srcinfo)

	k := 0
	for base, dir := range pkgBuildDirs {
		text.OperationInfoln(gotext.Get("(%d/%d) Parsing SRCINFO: %s", k+1, len(pkgBuildDirs), text.Cyan(base)))

		pkgbuild, err := gosrc.ParseFile(filepath.Join(dir, ".SRCINFO"))
		if err != nil {
			if !errIsFatal {
				text.Warnln(gotext.Get("failed to parse %s -- skipping: %s", base, err))
				continue
			}

			return nil, errors.New(gotext.Get("failed to parse %s: %s", base, err))
		}

		srcinfos[base] = pkgbuild
		k++
	}

	return srcinfos, nil
}
