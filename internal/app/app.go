package app

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/codenoid/minikv"
	tele "gopkg.in/telebot.v3"
	"toshiki-captcha-bot/internal/cli"
	"toshiki-captcha-bot/internal/commandscope"
	"toshiki-captcha-bot/internal/settings"
	"toshiki-captcha-bot/internal/version"
)

var (
	bot *tele.Bot

	cfg = settings.DefaultRuntimeConfig()
	db  *minikv.KV

	commandScopeStatePath = commandscope.PathForConfig(settings.DefaultConfigPath)
)

// Main bootstraps and runs the Telegram bot process.
func Main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("[captcha-bot] ")

	opts, err := cli.ParseArgs(os.Args[1:], settings.DefaultConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n%s", err, cli.UsageText(settings.DefaultConfigPath))
		os.Exit(2)
	}

	if opts.ShowHelp {
		fmt.Print(cli.UsageText(settings.DefaultConfigPath))
		return
	}
	if opts.ShowVersion {
		fmt.Print(version.Text())
		return
	}

	commandScopeStatePath = commandscope.PathForConfig(opts.ConfigPath)

	loadedCfg, err := settings.Load(opts.ConfigPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	cfg = loadedCfg
	db = minikv.New(cfg.Captcha.Expiration, cfg.Captcha.CleanupInterval)
	log.Printf(
		"Loaded config path=%q poll_timeout=%s request_timeout=%s public_mode=%t admin_user_ids=%d groups=%d topic_mappings=%d captcha_expiration=%s max_failures=%d",
		opts.ConfigPath,
		cfg.Bot.PollTimeout,
		cfg.Bot.RequestTimeout,
		cfg.IsPublicMode(),
		cfg.AdminUserCount(),
		cfg.GroupCount(),
		cfg.TopicMappingCount(),
		cfg.Captcha.Expiration,
		cfg.Captcha.MaxFailures,
	)

	// listen for janitor expiration removal ( 5*time.Second )
	db.OnEvicted(onEvicted)

	b, err := tele.NewBot(tele.Settings{
		Token:  cfg.Bot.Token,
		Poller: &tele.LongPoller{Timeout: cfg.Bot.PollTimeout},
		Client: &http.Client{Timeout: cfg.Bot.RequestTimeout},
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
