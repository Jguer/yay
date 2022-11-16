package db

func ArchIsSupported(alpmArch []string, arch string) bool {
	if arch == "any" {
		return true
	}

	for _, a := range alpmArch {
		if a == arch {
			return true
		}
	}

	return false
}
