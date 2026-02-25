package app

import (
	"fmt"
	"log"
	"os"

	"github.com/codenoid/minikv"
	tele "gopkg.in/telebot.v3"
	"toshiki-captcha-bot/internal/cli"
	"toshiki-captcha-bot/internal/commandscope"
	"toshiki-captcha-bot/internal/version"
)

var (
	bot *tele.Bot

	cfg = defaultRuntimeConfig()
	db  *minikv.KV

	commandScopeStatePath = commandscope.PathForConfig(defaultConfigPath)
)

// Main bootstraps and runs the Telegram bot process.
func Main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("[captcha-bot] ")

	opts, err := cli.ParseArgs(os.Args[1:], defaultConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n%s", err, cli.UsageText(defaultConfigPath))
		os.Exit(2)
	}

	if opts.ShowHelp {
		fmt.Print(cli.UsageText(defaultConfigPath))
		return
	}
	if opts.ShowVersion {
		fmt.Print(version.Text())
		return
	}

	commandScopeStatePath = commandscope.PathForConfig(opts.ConfigPath)

	loadedCfg, err := loadConfig(opts.ConfigPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	cfg = loadedCfg
	db = minikv.New(cfg.Captcha.Expiration, cfg.Captcha.CleanupInterval)
	log.Printf(
		"Loaded config path=%q poll_timeout=%s public_mode=%t admin_user_ids=%d groups=%d topic_mappings=%d captcha_expiration=%s max_failures=%d",
		opts.ConfigPath,
		cfg.Bot.PollTimeout,
		cfg.isPublicMode(),
		len(cfg.Bot.adminUsers),
		len(cfg.Groups),
		len(cfg.groupTopics),
		cfg.Captcha.Expiration,
		cfg.Captcha.MaxFailures,
	)

	// listen for janitor expiration removal ( 5*time.Second )
	db.OnEvicted(onEvicted)

	b, err := tele.NewBot(tele.Settings{
		Token:  cfg.Bot.Token,
		Poller: &tele.LongPoller{Timeout: cfg.Bot.PollTimeout},
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Bot initialized username=@%s id=%d", b.Me.Username, b.Me.ID)

	bot = b
	syncBotCommands(b)

	b.Handle("/help", onHelp)
	b.Handle("/version", onVersion)
	b.Handle("/ping", onPing)
	b.Handle("/testcaptcha", onTestCaptcha)
	b.Handle(tele.OnAddedToGroup, onAddedToGroup)
	b.Handle(tele.OnUserJoined, onJoin)
	b.Handle(tele.OnCallback, handleAnswer)
	b.Handle(tele.OnUserLeft, onUserLeft)

	log.Printf("Bot started and polling updates")
	b.Start()
}
