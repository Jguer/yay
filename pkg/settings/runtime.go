package settings

import (
	"net/http"

	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/query"
	"github.com/Jguer/yay/v11/pkg/settings/exe"
	"github.com/Jguer/yay/v11/pkg/settings/parser"
	"github.com/Jguer/yay/v11/pkg/vcs"

	"github.com/Jguer/aur"
	"github.com/Jguer/aur/metadata"
	"github.com/Jguer/votar/pkg/vote"
	"github.com/Morganamilo/go-pacmanconf"
)

type Runtime struct {
	Mode           parser.TargetMode
	QueryBuilder   query.Builder
	Version        string // current version of yay
	SaveConfig     bool
	CompletionPath string
	ConfigPath     string
	PacmanConf     *pacmanconf.Config
	VCSStore       *vcs.InfoStore
	CmdBuilder     exe.ICmdBuilder
	HTTPClient     *http.Client
	AURClient      *aur.Client
	VoteClient     *vote.Client
	AURCache       *metadata.Client
	DBExecutor     db.Executor
}
