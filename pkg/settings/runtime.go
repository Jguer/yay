package settings

type TargetMode int

const (
	ModeAny TargetMode = iota
	ModeAUR
	ModeRepo
)

type Runtime struct {
	Mode       TargetMode
	SaveConfig bool
}
