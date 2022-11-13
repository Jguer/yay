package settings

import (
	"net/http"

	"github.com/Morganamilo/go-pacmanconf"

	"github.com/Jguer/aur"
	"github.com/Jguer/votar/pkg/vote"

	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/metadata"
	"github.com/Jguer/yay/v11/pkg/query"
	"github.com/Jguer/yay/v11/pkg/settings/exe"
	"github.com/Jguer/yay/v11/pkg/settings/parser"
	"github.com/Jguer/yay/v11/pkg/vcs"
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
	AURCache       *metadata.AURCache
	DBExecutor     db.Executor
}
