package policy

import (
	"strings"

	tele "gopkg.in/telebot.v3"
	"toshiki-captcha-bot/internal/settings"
)

func IsGroupChat(chat *tele.Chat) bool {
	if chat == nil {
		return false
	}
	return chat.Type == tele.ChatGroup || chat.Type == tele.ChatSuperGroup
}

func IsPublicGroupChat(chat *tele.Chat) bool {
	if !IsGroupChat(chat) {
		return false
	}
	return strings.TrimSpace(chat.Username) != ""
}

func IsAllowedGroupChat(chat *tele.Chat, config settings.RuntimeConfig) bool {
	if !IsGroupChat(chat) {
		return false
	}
	if !IsPublicGroupChat(chat) {
		return false
	}
	if config.IsPublicMode() {
		return true
	}

	return config.IsAllowedPublicGroupUsername(chat.Username)
}

func IsAllowedCommandChat(chat *tele.Chat, config settings.RuntimeConfig) bool {
	if chat == nil {
		return false
	}
	if chat.Type == tele.ChatPrivate {
		return true
	}
	if IsGroupChat(chat) {
		return IsAllowedGroupChat(chat, config)
	}
	return false
}

func IsAuthorizedGroupChat(chat *tele.Chat, config settings.RuntimeConfig) bool {
	if !IsGroupChat(chat) {
		return false
	}
	return IsAllowedGroupChat(chat, config)
}

func IsAllowedUserID(userID int64, config settings.RuntimeConfig) bool {
	return config.HasAdminUser(userID)
}
