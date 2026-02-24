package main

import (
	"fmt"
	"log"
	"os"

	"github.com/codenoid/minikv"
	tele "gopkg.in/telebot.v3"
)

var (
	bot *tele.Bot

	cfg = defaultRuntimeConfig()
	db  = minikv.New(cfg.Captcha.Expiration, cfg.Captcha.CleanupInterval)
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("[captcha-bot] ")

	opts, err := parseCLIArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n%s", err, usageText())
		os.Exit(2)
	}

	if opts.showHelp {
		fmt.Print(usageText())
		return
	}
	if opts.showVersion {
		fmt.Print(versionText())
		return
	}

	loadedCfg, err := loadConfig(opts.configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	cfg = loadedCfg
	db = minikv.New(cfg.Captcha.Expiration, cfg.Captcha.CleanupInterval)
	log.Printf(
		"Loaded config path=%q poll_timeout=%s topic_link=%q topic_thread_id=%d captcha_expiration=%s max_failures=%d",
		opts.configPath,
		cfg.Bot.PollTimeout,
		cfg.Bot.TopicLink,
		cfg.Bot.TopicThreadID,
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

	b.Handle("/testcaptcha", onJoin)
	b.Handle(tele.OnUserJoined, onJoin)
	b.Handle(tele.OnCallback, handleAnswer)
	b.Handle(tele.OnUserLeft, func(c tele.Context) error {
		if err := c.Delete(); err != nil {
			log.Printf("warn: failed to delete leave-event message chat_id=%d err=%v", c.Chat().ID, err)
		}
		db.Delete(fmt.Sprint(c.Sender().ID))
		log.Printf("User left user_id=%d chat_id=%d", c.Sender().ID, c.Chat().ID)
		return nil
	})

	log.Printf("Bot started and polling updates")
	b.Start()
}
