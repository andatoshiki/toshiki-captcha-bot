package main

import (
	"fmt"
	"runtime"
	"strings"
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
		"toshiki-captcha-bot\nVersion: %s\nCommit: %s\nBuild time: %s\nGo: %s %s/%s\n",
		Version,
		Commit,
		BuildTime,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH,
	)
}

func versionTextMarkdown() string {
	runtimeInfo := fmt.Sprintf("%s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	return fmt.Sprintf(
		"toshiki-captcha-bot\nVersion: %s\nCommit: %s\nBuild time: %s\nGo: %s\n",
		inlineCode(Version),
		inlineCode(Commit),
		inlineCode(BuildTime),
		inlineCode(runtimeInfo),
	)
}

func inlineCode(value string) string {
	escaped := strings.ReplaceAll(value, "`", "\\`")
	return "`" + escaped + "`"
}
