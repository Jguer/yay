package parser

type TargetMode int

const (
	ModeAny TargetMode = iota
	ModeAUR
	ModeRepo
)
