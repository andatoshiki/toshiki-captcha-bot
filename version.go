package main

import (
	"fmt"
	"runtime"
)

var (
	// Version is the app version. Override via -ldflags "-X main.Version=x.y.z".
	Version = "dev"
	// Commit is the Git commit hash. Override via -ldflags "-X main.Commit=<sha>".
	Commit = "none"
	// BuildTime is build timestamp. Override via -ldflags "-X main.BuildTime=<ts>".
	BuildTime = "unknown"
)

func versionText() string {
	return fmt.Sprintf(
		"toshiki-captcha-bot\nversion: %s\ncommit: %s\nbuild_time: %s\ngo: %s %s/%s\n",
		Version,
		Commit,
		BuildTime,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH,
	)
}
