package dep

import (
	"context"
	"fmt"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/dep/topo"
	"github.com/Jguer/yay/v12/pkg/text"
)

const (
	sourceAUR          = "aur"
	sourceCacheSRCINFO = "srcinfo"
)

type InstallInfo struct {
	Source       Source
	Reason       Reason
	Version      string
	LocalVersion string
	SrcinfoPath  *string
	AURBase      *string
	SyncDBName   *string

	IsGroup bool
	Upgrade bool
	Devel   bool
}

func (i *InstallInfo) String() string {
	return fmt.Sprintf("InstallInfo{Source: %v, Reason: %v}", i.Source, i.Reason)
}

type (
	Reason int
	Source int
)

func (r Reason) String() string {
	return ReasonNames[r]
}

func (s Source) String() string {
	return SourceNames[s]
}

const (
	Explicit Reason = iota // 0
	Dep                    // 1
	MakeDep                // 2
	CheckDep               // 3
)

var ReasonNames = map[Reason]string{
	Explicit: gotext.Get("Explicit"),
	Dep:      gotext.Get("Dependency"),
	MakeDep:  gotext.Get("Make Dependency"),
	CheckDep: gotext.Get("Check Dependency"),
}

const (
	AUR Source = iota
	Sync
	Local
	SrcInfo
	Missing
)

var SourceNames = map[Source]string{
	AUR:     gotext.Get("AUR"),
	Sync:    gotext.Get("Sync"),
	Local:   gotext.Get("Local"),
	SrcInfo: gotext.Get("SRCINFO"),
	Missing: gotext.Get("Missing"),
}

var bgColorMap = map[Source]string{
	AUR:     "lightblue",
	Sync:    "lemonchiffon",
	Local:   "darkolivegreen1",
	Missing: "tomato",
}

var colorMap = map[Reason]string{
	Explicit: "black",
	Dep:      "deeppink",
	MakeDep:  "navyblue",
	CheckDep: "forestgreen",
}

type SourceHandler interface {
	Graph(ctx context.Context, graph *topo.Graph[string, *InstallInfo]) error
	Test(target Target) bool
}

type UpgradeHandler interface {
	GraphUpgrades(ctx context.Context, graph *topo.Graph[string, *InstallInfo],
		enableDowngrade bool, filter Filter,
	)
}

type Grapher struct {
	logger          *text.Logger
	handlers        map[string][]SourceHandler
	upgradeHandlers map[string][]UpgradeHandler
}

func NewGrapher(logger *text.Logger) *Grapher {
	grapher := &Grapher{
		logger:   logger,
		handlers: make(map[string][]SourceHandler),
	}

	return grapher
}

func NewGraph() *topo.Graph[string, *InstallInfo] {
	return topo.New[string, *InstallInfo]()
}

func (g *Grapher) RegisterSourceHandler(handler SourceHandler, source string) {
	g.handlers[source] = append(g.handlers[source], handler)

	if upgradeHandler, ok := handler.(UpgradeHandler); ok {
		g.upgradeHandlers[source] = append(g.upgradeHandlers[source], upgradeHandler)
	}
}

func (g *Grapher) GraphFromTargets(ctx context.Context,
	graph *topo.Graph[string, *InstallInfo], targets []string,
) (*topo.Graph[string, *InstallInfo], error) {
	if graph == nil {
		graph = NewGraph()
	}

	sources := mapset.NewThreadUnsafeSetFromMapKeys[string, []SourceHandler](g.handlers)

nextTarget:
	for _, targetString := range targets {
		target := ToTarget(targetString)

		for _, handler := range g.handlers[target.DB] {
			if handler.Test(target) {
				continue nextTarget
			}
		}

		g.logger.Errorln(gotext.Get("No package found for"), " ", target)
	}

	for source := range sources.Iter() {
		for _, handler := range g.handlers[source] {
			if err := handler.Graph(ctx, graph); err != nil {
				g.logger.Errorln(gotext.Get("Error graphing targets"), ":", err)
			}
		}
	}

	return graph, nil
}

// Filter decides if specific package should be included in theincluded in the  results.
type Filter func(*db.Upgrade) bool

func (g *Grapher) GraphUpgrades(ctx context.Context,
	graph *topo.Graph[string, *InstallInfo], enableDowngrade bool,
) (*topo.Graph[string, *InstallInfo], error) {
	if graph == nil {
		graph = NewGraph()
	}

	return graph, nil
}
