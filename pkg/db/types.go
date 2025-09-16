package db

import "slices"

func ArchIsSupported(alpmArch []string, arch string) bool {
	if arch == "any" {
		return true
	}

	return slices.Contains(alpmArch, arch)
}
