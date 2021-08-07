package settings

import (
	"net/http"

	"github.com/Morganamilo/go-pacmanconf"

	"github.com/Jguer/aur"

	"github.com/Jguer/yay/v10/pkg/settings/exe"
	"github.com/Jguer/yay/v10/pkg/settings/parser"
	"github.com/Jguer/yay/v10/pkg/vcs"
)

type Runtime struct {
	Mode           parser.TargetMode
	SaveConfig     bool
	CompletionPath string
	ConfigPath     string
	PacmanConf     *pacmanconf.Config
	VCSStore       *vcs.InfoStore
	CmdBuilder     exe.ICmdBuilder
	CmdRunner      exe.Runner
	HTTPClient     *http.Client
	AURClient      *aur.Client
}
