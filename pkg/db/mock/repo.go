package mock

import (
	"time"

	"github.com/Jguer/go-alpm/v2"
)

type Package struct {
	PBase         string
	PBuildDate    time.Time
	PDB           alpm.IDB
	PDescription  string
	PISize        int64
	PName         string
	PShouldIgnore bool
	PSize         int64
	PVersion      string
	PReason       alpm.PkgReason
}

func (p *Package) Base() string {
	return p.PBase
}

func (p *Package) BuildDate() time.Time {
	return p.PBuildDate
}

func (p *Package) DB() alpm.IDB {
	return p.PDB
}

func (p *Package) Description() string {
	return p.PDescription
}

func (p *Package) ISize() int64 {
	return p.PISize
}

func (p *Package) Name() string {
	return p.PName
}

func (p *Package) ShouldIgnore() bool {
	return p.PShouldIgnore
}

func (p *Package) Size() int64 {
	return p.PSize
}

func (p *Package) Version() string {
	return p.PVersion
}

func (p *Package) Reason() alpm.PkgReason {
	return p.PReason
}
