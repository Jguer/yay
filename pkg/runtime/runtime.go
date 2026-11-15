package runtime

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v13/pkg/aursource"
	"github.com/Jguer/yay/v13/pkg/download"
	"github.com/Jguer/yay/v13/pkg/query"
	"github.com/Jguer/yay/v13/pkg/settings"
	"github.com/Jguer/yay/v13/pkg/settings/exe"
	settingslua "github.com/Jguer/yay/v13/pkg/settings/lua"
	"github.com/Jguer/yay/v13/pkg/settings/parser"
	"github.com/Jguer/yay/v13/pkg/text"
	"github.com/Jguer/yay/v13/pkg/vcs"

	"github.com/Jguer/aur"
	"github.com/Jguer/aur/metadata"
	"github.com/Jguer/aur/rpc"
	"github.com/Jguer/votar/pkg/vote"
	"github.com/Morganamilo/go-pacmanconf"

	"golang.org/x/net/proxy"
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
	AURSources   *aursource.Client // resolves which AUR source a package came from
	Logger       *text.Logger
	Lua          *settingslua.Engine // not goroutine-safe
}

func NewRuntime(cfg *settings.Configuration, cmdArgs *parser.Arguments, version string) (*Runtime, error) {
	logger := text.NewLogger(os.Stdout, os.Stderr, os.Stdin, cfg.Debug, "runtime")
	runner := exe.NewOSRunner(logger.Child("runner"))

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.IdleConnTimeout = 90 * time.Second
	transport.TLSHandshakeTimeout = 30 * time.Second
	transport.ResponseHeaderTimeout = 30 * time.Second
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 10

	if socks5Proxy := os.Getenv("SOCKS5_PROXY"); socks5Proxy != "" {
		dialer, err := proxy.SOCKS5("tcp", socks5Proxy, nil, proxy.Direct)
		if err != nil {
			return nil, err
		}

		contextDialer, ok := dialer.(proxy.ContextDialer)
		if !ok {
			return nil, fmt.Errorf("SOCKS5 dialer does not support DialContext")
		}
		transport.DialContext = contextDialer.DialContext
	}

	httpClient := &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
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

	sources := cfg.AURSources()
	clients := make([]aur.QueryClient, 0, len(sources))

	for i, src := range sources {
		client, err := newAURQueryClient(cfg, httpClient, userAgentFn, logger, src, i)
		if err != nil {
			return nil, err
		}

		clients = append(clients, client)
	}

	aurSources := aursource.New(sources, clients, logger.Child("aursource"))

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
		aurSources,
		logger.Child("mixed.querybuilder"), cfg.SortBy,
		cfg.Mode, cfg.SearchBy,
		cfg.BottomUp, cfg.SingleLineResults, cfg.SeparateSources)

	run := &Runtime{
		Cfg:          cfg,
		QueryBuilder: queryBuilder,
		PacmanConf:   pacmanConf,
		VCSStore:     vcsStore,
		CmdBuilder:   cmdBuilder,
		HTTPClient:   httpClient,
		VoteClient:   voteClient,
		AURClient:    aurSources,
		AURSources:   aurSources,
		Logger:       logger,
	}

	return run, nil
}

// newAURQueryClient builds the query client for one AUR source. The first
// source keeps the historical aur.json metadata cache path; additional
// sources get their own cache file.
func newAURQueryClient(cfg *settings.Configuration, httpClient *http.Client,
	editor func(context.Context, *http.Request) error, logger *text.Logger,
	src settings.AURSource, idx int,
) (aur.QueryClient, error) {
	if cfg.UseRPC {
		return rpc.NewClient(
			rpc.WithHTTPClient(httpClient),
			rpc.WithBaseURL(src.RPCURL),
			rpc.WithRequestEditorFn(editor),
			rpc.WithLogFn(logger.Debugln))
	}

	cacheFile := "aur.json"
	if idx > 0 {
		cacheFile = "aur-" + cacheFileSafe(src.Name) + ".json"
	}

	client, err := metadata.New(
		metadata.WithHTTPClient(httpClient),
		metadata.WithCacheFilePath(filepath.Join(cfg.BuildDir, cacheFile)),
		metadata.WithRequestEditorFn(editor),
		metadata.WithBaseURL(src.URL),
		metadata.WithDebugLogger(logger.Debugln),
	)
	if err != nil {
		return nil, fmt.Errorf(gotext.Get("failed to retrieve aur Cache")+": %w", err)
	}

	return client, nil
}

func cacheFileSafe(name string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		default:
			return '_'
		}
	}, name)
}

// AURURLs returns the resolver used to pick AUR download URLs per package.
func (r *Runtime) AURURLs() download.AURURLResolver {
	if r.AURSources != nil {
		return r.AURSources
	}

	return download.StaticAURURLs(r.Cfg.AURBaseURLs())
}

// AURSourceFor returns the AUR source a package was resolved from.
func (r *Runtime) AURSourceFor(name string) settings.AURSource {
	if r.AURSources != nil {
		return r.AURSources.SourceFor(name)
	}

	return r.Cfg.AURSources()[0]
}
