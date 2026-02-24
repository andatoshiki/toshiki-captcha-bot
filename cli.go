package main

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

type cliOptions struct {
	configPath  string
	showVersion bool
	showHelp    bool
}

func parseCLIArgs(args []string) (cliOptions, error) {
	opts := cliOptions{
		configPath: defaultConfigPath,
	}

	fs := flag.NewFlagSet("toshiki-captcha-bot", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	fs.StringVar(&opts.configPath, "c", defaultConfigPath, "Path to YAML config file")
	fs.StringVar(&opts.configPath, "config", defaultConfigPath, "Path to YAML config file")
	fs.BoolVar(&opts.showVersion, "v", false, "Print version and exit")
	fs.BoolVar(&opts.showVersion, "version", false, "Print version and exit")
	fs.BoolVar(&opts.showHelp, "h", false, "Show help and exit")
	fs.BoolVar(&opts.showHelp, "help", false, "Show help and exit")

	if err := fs.Parse(args); err != nil {
		return cliOptions{}, err
	}
	if len(fs.Args()) > 0 {
		return cliOptions{}, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}

	return opts, nil
}

func usageText() string {
	return fmt.Sprintf(`Telegram CAPTCHA bot

Usage:
  toshiki-captcha-bot [options]

Options:
  -c, --config <path>   YAML configuration path (default: %s)
  -v, --version         Print version and exit
  -h, --help            Show this help and exit
`, defaultConfigPath)
}
