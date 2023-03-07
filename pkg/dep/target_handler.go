package dep

import "github.com/Jguer/yay/v12/pkg/text"

type Target struct {
	DB      string
	Name    string
	Mod     string
	Version string
}

func ToTarget(pkg string) Target {
	dbName, depString := text.SplitDBFromName(pkg)
	name, mod, depVersion := splitDep(depString)

	return Target{
		DB:      dbName,
		Name:    name,
		Mod:     mod,
		Version: depVersion,
	}
}

func (t Target) DepString() string {
	return t.Name + t.Mod + t.Version
}

func (t Target) String() string {
	if t.DB != "" {
		return t.DB + "/" + t.DepString()
	}

	return t.DepString()
}
