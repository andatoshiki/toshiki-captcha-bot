package main

import "toshiki-captcha-bot/internal/app"

var (
	// Version is the app version. Override via -ldflags "-X main.Version=x.y.z".
	Version = "dev"
	// Commit is the Git commit hash. Override via -ldflags "-X main.Commit=<sha>".
	Commit = "none"
	// BuildTime is build timestamp. Override via -ldflags "-X main.BuildTime=<ts>".
	BuildTime = "unknown"
)

func init() {
	app.Version = Version
	app.Commit = Commit
	app.BuildTime = BuildTime
}

func main() {
	app.Main()
}
