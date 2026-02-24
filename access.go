package main

import (
	"fmt"
	"log"

	tele "gopkg.in/telebot.v3"
)

func isContextAuthorized(c tele.Context) bool {
	if cfg.Bot.Public {
		return true
	}
	if c == nil {
		return false
	}

	if sender := c.Sender(); sender != nil {
		if _, ok := cfg.Bot.allowedUsers[sender.ID]; ok {
			return true
		}
	}

	chat := c.Chat()
	if chat == nil || bot == nil {
		return false
	}

	admins, err := bot.AdminsOf(chat)
	if err != nil {
		log.Printf("warn: failed to resolve chat admins chat_id=%d err=%v", chat.ID, err)
		return false
	}

	for _, admin := range admins {
		if admin.User == nil {
			continue
		}
		if _, ok := cfg.Bot.allowedUsers[admin.User.ID]; ok {
			return true
		}
	}

	return false
}

func isAllowedUserID(userID int64) bool {
	if userID <= 0 {
		return false
	}
	_, ok := cfg.Bot.allowedUsers[userID]
	return ok
}

func isSenderAllowed(c tele.Context) bool {
	if c == nil || c.Sender() == nil {
		return false
	}
	return isAllowedUserID(c.Sender().ID)
}

func logAccessDenied(c tele.Context, event string) {
	var chatID int64
	var userID int64
	if c != nil && c.Chat() != nil {
		chatID = c.Chat().ID
	}
	if c != nil && c.Sender() != nil {
		userID = c.Sender().ID
	}
	log.Printf("Access denied event=%s chat_id=%d user_id=%d public=%t", event, chatID, userID, cfg.Bot.Public)
}

func onAddedToGroup(c tele.Context) error {
	if isContextAuthorized(c) {
		return nil
	}
	logAccessDenied(c, "added_to_group")
	if c.Chat() == nil {
		return nil
	}
	if err := bot.Leave(c.Chat()); err != nil {
		log.Printf("warn: failed to leave unauthorized chat chat_id=%d err=%v", c.Chat().ID, err)
	}
	return nil
}

func onUserLeft(c tele.Context) error {
	if c == nil || c.Chat() == nil {
		return nil
	}
	if !isContextAuthorized(c) {
		logAccessDenied(c, "user_left")
		return nil
	}

	if err := c.Delete(); err != nil {
		log.Printf("warn: failed to delete leave-event message chat_id=%d err=%v", c.Chat().ID, err)
	}

	if c.Sender() != nil {
		db.Delete(fmt.Sprintf("%v-%v", c.Sender().ID, c.Chat().ID))
		log.Printf("User left user_id=%d chat_id=%d", c.Sender().ID, c.Chat().ID)
	}

	return nil
}
