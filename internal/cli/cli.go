package cli

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

type Options struct {
	ConfigPath  string
	ShowVersion bool
	ShowHelp    bool
}

func ParseArgs(args []string, defaultConfigPath string) (Options, error) {
	opts := Options{
		ConfigPath: defaultConfigPath,
	}

	fs := flag.NewFlagSet("toshiki-captcha-bot", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	fs.StringVar(&opts.ConfigPath, "c", defaultConfigPath, "Path to YAML config file")
	fs.StringVar(&opts.ConfigPath, "config", defaultConfigPath, "Path to YAML config file")
	fs.BoolVar(&opts.ShowVersion, "v", false, "Print version and exit")
	fs.BoolVar(&opts.ShowVersion, "version", false, "Print version and exit")
	fs.BoolVar(&opts.ShowHelp, "h", false, "Show help and exit")
	fs.BoolVar(&opts.ShowHelp, "help", false, "Show help and exit")

	if err := fs.Parse(args); err != nil {
		return Options{}, err
	}
	if len(fs.Args()) > 0 {
		return Options{}, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}

	return opts, nil
}

func UsageText(defaultConfigPath string) string {
	return fmt.Sprintf(`Telegram CAPTCHA bot

Usage:
  toshiki-captcha-bot [options]

Options:
  -c, --config <path>   YAML configuration path (default: %s)
  -v, --version         Print version and exit
  -h, --help            Show this help and exit
`, defaultConfigPath)
}
