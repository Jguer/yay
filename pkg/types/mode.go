package types

// TargetMode describes the target sources to use in functions that mix AUR and repo
type TargetMode int

const (
	// AUR mode, use AUR targets
	AUR TargetMode = iota
	// Repo mode, use Repo targets
	Repo
	// Any mode, use Repo or AUR targets
	Any
)

// IsAnyOrAUR returns true if TargetMode is set to Any or to AUR
func (t TargetMode) IsAnyOrAUR() bool {
	return t == Any || t == AUR
}

// IsAnyOrRepo returns true if TargetMode is set to Any or to Repo
func (t TargetMode) IsAnyOrRepo() bool {
	return t == Any || t == Repo
}

// IsAUR returns true if TargetMode is set to AUR
func (t TargetMode) IsAUR() bool {
	return t == AUR
}

// IsRepo returns true if TargetMode is set to Repo
func (t TargetMode) IsRepo() bool {
	return t == Repo
}
