package upgrade

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v13/pkg/dep"
	"github.com/Jguer/yay/v13/pkg/dep/topo"
	"github.com/Jguer/yay/v13/pkg/settings/exe"
	"github.com/Jguer/yay/v13/pkg/text"
)

// MinAgeAction describes how minage adjusted a package.
type MinAgeAction int

const (
	MinAgeKept MinAgeAction = iota
	MinAgeDowngraded
	MinAgeSkipped
)

// MinAgeEntry records a minage decision for one package.
type MinAgeEntry struct {
	Action        MinAgeAction
	Name          string
	Version       string
	RemoteVersion string
	Age           time.Duration
	Trigger       string
	SourcePath    string
}

// MinAgeAdjuster supplies paths and commands for downgrade lookups.
type MinAgeAdjuster struct {
	CacheDirs  []string
	BuildDir   string
	AURURL     string
	PacmanBin  string
	CmdBuilder exe.ICmdBuilder
	// DryRun skips cache and AUR downgrade lookups (keep/skip from metadata only).
	DryRun bool
}

func packageAge(lastModified int64) time.Duration {
	if lastModified == 0 {
		return 0
	}

	return text.NowFunc().Sub(time.Unix(lastModified, 0))
}

func packageTooYoung(lastModified int64, minAgeDays int) bool {
	if minAgeDays <= 0 || lastModified == 0 {
		return false
	}

	return packageAge(lastModified) < time.Duration(minAgeDays)*24*time.Hour
}

// ApplyMinAge keeps, downgrades, or removes packages younger than minAgeDays.
// pacmanIgnore lists repo packages pacman -u must skip (kept or cache-installed).
func ApplyMinAge(ctx context.Context, graph *topo.Graph[string, *dep.InstallInfo],
	minAgeDays int, adj *MinAgeAdjuster,
) (report []MinAgeEntry, pacmanIgnore []string) {
	if minAgeDays <= 0 || graph == nil || adj == nil {
		return nil, nil
	}

	seen := make(map[string]struct{})

	for {
		trigger, triggerInfo := findYoungPackage(graph, minAgeDays)
		if trigger == "" {
			break
		}

		if triggerInfo.Upgrade && triggerInfo.LocalVersion != "" {
			remote := triggerInfo.Version
			for _, name := range graph.Prune(trigger) {
				if name == trigger {
					report = append(report, MinAgeEntry{
						Action:        MinAgeKept,
						Name:          name,
						Version:       triggerInfo.LocalVersion,
						RemoteVersion: remote,
						Age:           packageAge(triggerInfo.LastModified),
					})
					pacmanIgnore = append(pacmanIgnore, name)
				} else {
					recordMinAgePrune(&report, seen, name, trigger, triggerInfo)
				}
			}

			continue
		}

		if !adj.DryRun {
			if path, version, built, ok := FindEligibleCachePackage(ctx, adj.CmdBuilder, adj.PacmanBin,
				adj.CacheDirs, trigger, minAgeDays); ok {
				oldRemote := triggerInfo.Version
				triggerInfo.Version = version
				triggerInfo.LastModified = built
				triggerInfo.PkgArchive = path
				triggerInfo.Upgrade = false
				report = append(report, MinAgeEntry{
					Action:        MinAgeDowngraded,
					Name:          trigger,
					Version:       version,
					RemoteVersion: oldRemote,
					Age:           packageAge(built),
					SourcePath:    path,
				})
				pacmanIgnore = append(pacmanIgnore, trigger)

				continue
			}

			if triggerInfo.Source == dep.AUR && triggerInfo.AURBase != "" && adj.CmdBuilder != nil {
				if gitRef, version, built, ok := findAURVersionBefore(ctx, adj.CmdBuilder,
					adj.AURURL, adj.BuildDir, triggerInfo.AURBase, minAgeDays); ok {
					oldRemote := triggerInfo.Version
					triggerInfo.Version = version
					triggerInfo.LastModified = built
					triggerInfo.AURGitRef = gitRef
					triggerInfo.Upgrade = false
					report = append(report, MinAgeEntry{
						Action:        MinAgeDowngraded,
						Name:          trigger,
						Version:       version,
						RemoteVersion: oldRemote,
						Age:           packageAge(built),
						SourcePath:    gitRef,
					})

					continue
				}
			}
		}

		for _, name := range graph.Prune(trigger) {
			recordMinAgePrune(&report, seen, name, trigger, triggerInfo)
		}
	}

	return report, pacmanIgnore
}

func findYoungPackage(graph *topo.Graph[string, *dep.InstallInfo], minAgeDays int) (string, *dep.InstallInfo) {
	var young []string

	_ = graph.ForEach(func(name string, info *dep.InstallInfo) error {
		if info.Source == dep.Local || info.Source == dep.Missing {
			return nil
		}

		if packageTooYoung(info.LastModified, minAgeDays) {
			young = append(young, name)
		}

		return nil
	})

	if len(young) == 0 {
		return "", nil
	}

	slices.Sort(young)
	trigger := young[0]

	return trigger, graph.GetNodeInfo(trigger).Value
}

func recordMinAgePrune(report *[]MinAgeEntry, seen map[string]struct{}, name, trigger string,
	triggerInfo *dep.InstallInfo,
) {
	if _, ok := seen[name]; ok {
		return
	}

	seen[name] = struct{}{}
	entry := MinAgeEntry{
		Action: MinAgeSkipped,
		Name:   name,
	}
	if name == trigger {
		entry.Age = packageAge(triggerInfo.LastModified)
	} else {
		entry.Trigger = trigger
	}

	*report = append(*report, entry)
}

func minAgeSectionHeader(action MinAgeAction, count, minAgeDays int) string {
	switch action {
	case MinAgeKept:
		return gotext.Get("%s kept at current version (remote newer than %d days).",
			gotext.GetN("package", "packages", count), minAgeDays)
	case MinAgeDowngraded:
		return gotext.Get("%s downgraded to meet minimum age (%d days).",
			gotext.GetN("package", "packages", count), minAgeDays)
	case MinAgeSkipped:
		return gotext.Get("%s skipped (no version old enough, minimum %d days).",
			gotext.GetN("package", "packages", count), minAgeDays)
	default:
		return ""
	}
}

// PrintMinAgeReport prints minage adjustments.
func PrintMinAgeReport(logger *text.Logger, report []MinAgeEntry, minAgeDays int) {
	if len(report) == 0 {
		return
	}

	counts := make(map[MinAgeAction]int, 3)
	for i := range report {
		counts[report[i].Action]++
	}

	for _, action := range []MinAgeAction{MinAgeKept, MinAgeDowngraded, MinAgeSkipped} {
		count := counts[action]
		if count == 0 {
			continue
		}

		logger.Printf("%s"+text.Bold(" %d ")+"%s\n", text.Bold(text.Cyan("::")),
			count, text.Bold(minAgeSectionHeader(action, count, minAgeDays)))
		printMinAgeEntries(logger, report, action, minAgeDays)
	}

	logger.Println()
}

func printMinAgeEntries(logger *text.Logger, report []MinAgeEntry, action MinAgeAction, minAgeDays int) {
	for i := range report {
		entry := &report[i]
		if entry.Action != action {
			continue
		}

		line := fmt.Sprintf("  %s", text.Bold(entry.Name))
		switch entry.Action {
		case MinAgeKept:
			line += " — " + gotext.Get("keeping %s (remote %s is %s old)",
				text.Bold(entry.Version), text.Bold(entry.RemoteVersion),
				text.Bold(text.FormatDuration(entry.Age)))
		case MinAgeDowngraded:
			line += " — " + gotext.Get("%s -> %s (%s old)",
				text.Bold(entry.RemoteVersion), text.Bold(entry.Version),
				text.Bold(text.FormatDuration(entry.Age)))
			if entry.SourcePath != "" && entry.SourcePath[0] == '/' {
				line += " [" + entry.SourcePath + "]"
			}
		case MinAgeSkipped:
			if entry.Trigger != "" {
				line += " — " + gotext.Get("skipped: dependency %s is younger than %d days",
					text.Bold(entry.Trigger), minAgeDays)
			} else {
				line += " — " + gotext.Get("package is %s old (minimum %d days)",
					text.Bold(text.FormatDuration(entry.Age)), minAgeDays)
			}
		}

		logger.Println(line)
	}
}
