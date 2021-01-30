package settings

import (
	"net/http"

	"github.com/Morganamilo/go-pacmanconf"

	"github.com/Jguer/yay/v10/pkg/settings/exe"
	"github.com/Jguer/yay/v10/pkg/vcs"
)

type TargetMode int

const (
	ModeAny TargetMode = iota
	ModeAUR
	ModeRepo
)

type Runtime struct {
	Mode           TargetMode
	SaveConfig     bool
	CompletionPath string
	ConfigPath     string
	PacmanConf     *pacmanconf.Config
	VCSStore       *vcs.InfoStore
	CmdBuilder     *exe.CmdBuilder
	CmdRunner      exe.Runner
	HTTPClient     *http.Client
}
