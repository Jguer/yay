package parser

type TargetMode int

const (
	ModeAny TargetMode = iota
	ModeAUR
	ModeRepo
)

func (mode TargetMode) AtLeastAUR() bool {
	return mode == ModeAny || mode == ModeAUR
}

func (mode TargetMode) AtLeastRepo() bool {
	return mode == ModeAny || mode == ModeRepo
}
