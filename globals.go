package main

import "github.com/Jguer/yay/v11/pkg/settings"

// Verbosity settings for search.
const (
	numberMenu = iota
	detailed
	minimal
)

var (
	yayVersion = "11.0.1"            // To be set by compiler.
	localePath = "/usr/share/locale" // To be set by compiler.
)

var config *settings.Configuration // YayConf holds the current config values for yay.
