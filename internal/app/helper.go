package app

import (
	"fmt"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"
	"toshiki-captcha-bot/internal/settings"
)

func genCaption(user *tele.User) string {
	desc := fmt.Sprintf(
		"Select all the emoji you see in the picture in exact left-to-right order."+
			"\n\n Max failure: %d mistake \n Duration: %s"+
			"\n\n Please leave group immediately if you are not ready with the bot",
		cfg.Captcha.MaxFailures,
		humanizeDuration(cfg.Captcha.Expiration),
	)

	if user == nil {
		return desc
	}

	displayName := strings.TrimSpace(user.FirstName)
	if displayName == "" {
		displayName = "user"
	}
	displayName = escapeTelegramMarkdown(displayName)

	mention := fmt.Sprintf(`[%v](tg://user?id=%v)`, displayName, user.ID)
	caption := fmt.Sprintf("%v, %v", mention, desc)
	return caption
}

func escapeTelegramMarkdown(text string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"`", "\\`",
	)
	return replacer.Replace(text)
}

func humanizeDuration(d time.Duration) string {
	if d%time.Hour == 0 && d >= time.Hour {
		hours := int(d / time.Hour)
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	}

	if d%time.Minute == 0 && d >= time.Minute {
		minutes := int(d / time.Minute)
		if minutes == 1 {
			return "1 minute"
		}
		return fmt.Sprintf("%d minutes", minutes)
	}

	if d%time.Second == 0 {
		seconds := int(d / time.Second)
		if seconds == 1 {
			return "1 second"
		}
		return fmt.Sprintf("%d seconds", seconds)
	}

	return d.String()
}

func buildSendOptionsWithTopic(parseMode tele.ParseMode, markup *tele.ReplyMarkup, topicID int) *tele.SendOptions {
	opts := &tele.SendOptions{
		ParseMode: parseMode,
		ThreadID:  topicID,
	}
	if markup != nil {
		opts.ReplyMarkup = markup
	}
	return opts
}

func topicThreadIDForChat(chat *tele.Chat) int {
	return resolveTopicThreadIDForChat(chat, cfg)
}

func resolveTopicThreadIDForChat(chat *tele.Chat, config settings.RuntimeConfig) int {
	if chat == nil || chat.Type == tele.ChatPrivate {
		return 0
	}

	// Public mode discards all groups topic configuration by design.
	return config.TopicForChatUsername(chat.Username)
}

func sendWithConfiguredTopic(chat *tele.Chat, what interface{}, parseMode tele.ParseMode, markup *tele.ReplyMarkup) (*tele.Message, error) {
	opts := buildSendOptionsWithTopic(parseMode, markup, topicThreadIDForChat(chat))
	return bot.Send(chat, what, opts)
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func removeRedundantSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func sanitizeName(s string) string {
	var result strings.Builder
	for i := 0; i < len(s); i++ {
		b := s[i]
		if ('a' <= b && b <= 'z') ||
			('A' <= b && b <= 'Z') ||
			('0' <= b && b <= '9') ||
			b == ' ' {
			result.WriteByte(b)
		}
	}
	clean := removeRedundantSpaces(result.String())
	return clean
}
