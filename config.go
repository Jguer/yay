package main

import (
	"github.com/Jguer/yay/v11/pkg/settings"
)

// Verbosity settings for search.
const (
	numberMenu = iota
	detailed
	minimal
)

var yayVersion = "11.0.1"

var localePath = "/usr/share/locale"

// YayConf holds the current config values for yay.
var config *settings.Configuration
