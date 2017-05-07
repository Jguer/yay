package main

import (
	"fmt"
	"io"
	"math"
	"os"
	"time"

	"github.com/jguer/yay/aur"
	"github.com/jguer/yay/config"
	pac "github.com/jguer/yay/pacman"
)

// Complete provides completion info for shells
func complete() (err error) {
	path := os.Getenv("HOME") + "/.cache/yay/aur_" + config.YayConf.Shell + ".cache"

	if info, err := os.Stat(path); os.IsNotExist(err) || time.Since(info.ModTime()).Hours() > 48 {
		os.MkdirAll(os.Getenv("HOME")+"/.cache/yay/", 0755)

		out, err := os.Create(path)
		if err != nil {
			return err
		}

		if aur.CreateAURList(out) != nil {
			defer os.Remove(path)
		}
		err = pac.CreatePackageList(out)

		out.Close()
		return err
	}

	in, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return err
	}
	defer in.Close()

	_, err = io.Copy(os.Stdout, in)
	return err
}

// Function by pyk https://github.com/pyk/byten
func index(s int64) float64 {
	x := math.Log(float64(s)) / math.Log(1024)
	return math.Floor(x)
}

// Function by pyk https://github.com/pyk/byten
func countSize(s int64, i float64) float64 {
	return float64(s) / math.Pow(1024, math.Floor(i))
}

// Size return a formated string from file size
// Function by pyk https://github.com/pyk/byten
func size(s int64) string {

	symbols := []string{"B", "KB", "MB", "GB", "TB", "PB", "EB"}
	i := index(s)
	if s < 10 {
		return fmt.Sprintf("%dB", s)
	}
	size := countSize(s, i)
	format := "%.0f"
	if size < 10 {
		format = "%.1f"
	}

	return fmt.Sprintf(format+"%s", size, symbols[int(i)])
}
