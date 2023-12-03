package runtime

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/query"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/exe"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"
	"github.com/Jguer/yay/v12/pkg/vcs"

	"github.com/Jguer/aur"
	"github.com/Jguer/aur/metadata"
	"github.com/Jguer/aur/rpc"
	"github.com/Jguer/votar/pkg/vote"
	"github.com/Morganamilo/go-pacmanconf"
)

type Runtime struct {
	Cfg          *settings.Configuration
	QueryBuilder query.Builder
	PacmanConf   *pacmanconf.Config
	VCSStore     vcs.Store
	CmdBuilder   exe.ICmdBuilder
	HTTPClient   *http.Client
	VoteClient   *vote.Client
	AURClient    aur.QueryClient
	Logger       *text.Logger
}

func NewRuntime(cfg *settings.Configuration, cmdArgs *parser.Arguments, version string) (*Runtime, error) {
	logger := text.NewLogger(os.Stdout, os.Stderr, os.Stdin, cfg.Debug, "runtime")
	runner := exe.NewOSRunner(logger.Child("runner"))

	httpClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	userAgent := fmt.Sprintf("Yay/%s", version)
	voteClient, errVote := vote.NewClient(vote.WithUserAgent(userAgent),
		vote.WithHTTPClient(httpClient))
	if errVote != nil {
		return nil, errVote
	}

	voteClient.SetCredentials(
		os.Getenv("AUR_USERNAME"),
		os.Getenv("AUR_PASSWORD"))

	userAgentFn := func(ctx context.Context, req *http.Request) error {
		req.Header.Set("User-Agent", userAgent)
		return nil
	}

	var aurCache aur.QueryClient
	aurCache, errAURCache := metadata.New(
		metadata.WithHTTPClient(httpClient),
		metadata.WithCacheFilePath(filepath.Join(cfg.BuildDir, "aur.json")),
		metadata.WithRequestEditorFn(userAgentFn),
		metadata.WithBaseURL(cfg.AURURL),
		metadata.WithDebugLogger(logger.Debugln),
	)
	if errAURCache != nil {
		return nil, fmt.Errorf(gotext.Get("failed to retrieve aur Cache")+": %w", errAURCache)
	}

	aurClient, errAUR := rpc.NewClient(
		rpc.WithHTTPClient(httpClient),
		rpc.WithBaseURL(cfg.AURRPCURL),
		rpc.WithRequestEditorFn(userAgentFn),
		rpc.WithLogFn(logger.Debugln))
	if errAUR != nil {
		return nil, errAUR
	}

	if cfg.UseRPC {
		aurCache = aurClient
	}

	pacmanConf, useColor, err := retrievePacmanConfig(cmdArgs, cfg.PacmanConf)
	if err != nil {
		return nil, err
	}

	// FIXME: get rid of global
	text.UseColor = useColor

	cmdBuilder := exe.NewCmdBuilder(cfg, runner, logger.Child("cmdbuilder"), pacmanConf.DBPath)

	vcsStore := vcs.NewInfoStore(
		cfg.VCSFilePath, cmdBuilder,
		logger.Child("vcs"))

	if err := vcsStore.Load(); err != nil {
		return nil, err
	}

	queryBuilder := query.NewSourceQueryBuilder(
		aurClient,
		logger.Child("mixed.querybuilder"), cfg.SortBy,
		cfg.Mode, cfg.SearchBy,
		cfg.BottomUp, cfg.SingleLineResults, cfg.SeparateSources, cfg.ShowPackageURLs)

	run := &Runtime{
		Cfg:          cfg,
		QueryBuilder: queryBuilder,
		PacmanConf:   pacmanConf,
		VCSStore:     vcsStore,
		CmdBuilder:   cmdBuilder,
		HTTPClient:   &http.Client{},
		VoteClient:   voteClient,
		AURClient:    aurCache,
		Logger:       logger,
	}

	return run, nil
}
