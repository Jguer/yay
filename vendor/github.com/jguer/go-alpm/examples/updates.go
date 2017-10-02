//
//
// Copyright (c) 2013 The go-alpm Authors
//
// MIT Licensed. See LICENSE for details.

package main

import (
	"fmt"
	"github.com/demizer/go-alpm"
	"log"
	"os"
)

func human(size int64) string {
	floatsize := float32(size)
	units := [...]string{"", "Ki", "Mi", "Gi", "Ti", "Pi", "Ei", "Zi", "Yi"}
	for _, unit := range units {
		if floatsize < 1024 {
			return fmt.Sprintf("%.1f %sB", floatsize, unit)
		}
		floatsize /= 1024
	}
	return fmt.Sprintf("%d%s", size, "B")
}

func upgrades(h *alpm.Handle) ([]alpm.Package, error) {
	localDb, err := h.LocalDb()
	if err != nil {
		return nil, err
	}

	syncDbs, err := h.SyncDbs()
	if err != nil {
		return nil, err
	}

	slice := []alpm.Package{}
	for _, pkg := range localDb.PkgCache().Slice() {
		newPkg := pkg.NewVersion(syncDbs)
		if newPkg != nil {
			slice = append(slice, *newPkg)
		}
	}
	return slice, nil
}

func main() {

	file, err := os.Open("/etc/pacman.conf")
	if err != nil {
		log.Fatalln(err)
	}
	conf, err := alpm.ParseConfig(file)
	if err != nil {
		log.Fatalln(err)
	}

	h, err := conf.CreateHandle()
	defer h.Release()
	if err != nil {
		log.Fatalln(err)
	}

	upgrades, err := upgrades(h)
	if err != nil {
		log.Fatalln(err)
	}

	var size int64 = 0
	for _, pkg := range upgrades {
		size += pkg.Size()
		fmt.Printf("%s %s -> %s\n", pkg.Name(), pkg.Version(),
			pkg.Version())
	}
	fmt.Printf("Total Download Size: %s\n", human(size))
}
