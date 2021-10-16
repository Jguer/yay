package main

import "github.com/Jguer/yay/v11/pkg/settings"

var (
	yayVersion = "11.0.1"            // To be set by compiler.
	localePath = "/usr/share/locale" // To be set by compiler.
)

var config *settings.Configuration // YayConf holds the current config values for yay.
