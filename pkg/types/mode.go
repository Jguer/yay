package types

type TargetMode int

const (
	AUR TargetMode = iota
	Repo
	Any
)

// AnyOrAUR checks if TargetMode is set to Any or to AUR
func (t TargetMode) IsAnyOrAUR() bool {
	return t == Any || t == AUR
}

func (t TargetMode) IsAnyOrRepo() bool {
	return t == Any || t == Repo
}

func (t TargetMode) IsAUR() bool {
	return t == AUR
}

func (t TargetMode) IsRepo() bool {
	return t == Repo
}
