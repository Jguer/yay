package settings

import (
	"net/http"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/query"
	"github.com/Jguer/yay/v12/pkg/settings/exe"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"
	"github.com/Jguer/yay/v12/pkg/vcs"

	"github.com/Jguer/aur"
	"github.com/Jguer/aur/rpc"
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
	VCSStore       vcs.Store
	CmdBuilder     exe.ICmdBuilder
	HTTPClient     *http.Client
	AURClient      *rpc.Client
	VoteClient     *vote.Client
	AURCache       aur.QueryClient
	DBExecutor     db.Executor
	Logger         *text.Logger
}
