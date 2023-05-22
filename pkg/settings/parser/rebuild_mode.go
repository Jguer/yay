package parser

type RebuildMode string

const (
	RebuildModeNo   RebuildMode = "no"
	RebuildModeYes  RebuildMode = "yes"
	RebuildModeTree RebuildMode = "tree"
	RebuildModeAll  RebuildMode = "all"
)
