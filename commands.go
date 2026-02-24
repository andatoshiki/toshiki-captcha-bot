package main

import (
	"log"
	"strings"

	tele "gopkg.in/telebot.v3"
)

const (
	projectURL  = "https://github.com/andatoshiki/toshiki-captcha-bot"
	authorInfo  = "Anda Toshiki @andatoshiki"
	licenseInfo = "MIT License"
)

func helpText() string {
	return strings.Join([]string{
		"Toshiki's Captcha Bot",
		"\"A lightweight Telegram gatekeeper built with telebot v3\"",
		"",
		"This bot protects group joins with an emoji captcha, restricts new users until captcha is solved, and bans users on max failures or timeout.",
		"",
		"commands:",
		"/help show this help message (public)",
		"/version show build and runtime version details (public)",
		"/ping check bot reachability and latency in ms (admin only)",
		"/testcaptcha manually trigger a captcha challenge (admin only)",
		"",
		"credits:",
		"author: " + authorInfo,
		"project: " + projectURL,
		"project licensed under " + licenseInfo,
	}, "\n")
}

func onHelp(c tele.Context) error {
	chatID, userID := commandContextIDs(c)
	log.Printf("Help requested chat_id=%d user_id=%d", chatID, userID)
	if err := c.Send(helpText()); err != nil {
		log.Printf("warn: failed to send help response chat_id=%d user_id=%d err=%v", chatID, userID, err)
	}
	return nil
}

func onVersion(c tele.Context) error {
	chatID, userID := commandContextIDs(c)
	log.Printf("Version requested chat_id=%d user_id=%d", chatID, userID)
	if err := c.Send(versionTextMarkdown(), tele.ModeMarkdown); err != nil {
		log.Printf("warn: failed to send version response chat_id=%d user_id=%d err=%v", chatID, userID, err)
	}
	return nil
}

func syncBotCommands(b *tele.Bot) {
	if b == nil {
		return
	}

	public := publicBotCommands()
	if err := b.SetCommands(public); err != nil {
		log.Printf("warn: failed to register default bot commands err=%v", err)
	} else {
		log.Printf("Bot commands updated scope=default count=%d", len(public))
	}

	admin := adminBotCommands()
	adminScope := tele.CommandScope{Type: tele.CommandScopeAllChatAdmin}
	if err := b.SetCommands(admin, adminScope); err != nil {
		log.Printf("warn: failed to register admin bot commands err=%v", err)
	} else {
		log.Printf("Bot commands updated scope=all_chat_administrators count=%d", len(admin))
	}
}

func publicBotCommands() []tele.Command {
	return []tele.Command{
		{Text: "help", Description: "show this help message"},
		{Text: "version", Description: "show build and runtime version details"},
	}
}

func adminBotCommands() []tele.Command {
	return []tele.Command{
		{Text: "help", Description: "show this help message"},
		{Text: "version", Description: "show build and runtime version details"},
		{Text: "ping", Description: "check bot reachability and latency in ms"},
		{Text: "testcaptcha", Description: "manually trigger a captcha challenge"},
	}
}

func commandContextIDs(c tele.Context) (int64, int64) {
	var chatID int64
	var userID int64
	if c != nil && c.Chat() != nil {
		chatID = c.Chat().ID
	}
	if c != nil && c.Sender() != nil {
		userID = c.Sender().ID
	}
	return chatID, userID
}
