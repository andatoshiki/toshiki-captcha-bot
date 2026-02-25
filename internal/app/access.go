package app

import (
	"fmt"
	"log"
	"strings"

	tele "gopkg.in/telebot.v3"
)

func isGroupChat(chat *tele.Chat) bool {
	if chat == nil {
		return false
	}
	return chat.Type == tele.ChatGroup || chat.Type == tele.ChatSuperGroup
}

func isPublicGroupChat(chat *tele.Chat) bool {
	if !isGroupChat(chat) {
		return false
	}
	return strings.TrimSpace(chat.Username) != ""
}

func isAllowedGroupChat(chat *tele.Chat) bool {
	return isAllowedGroupChatWithConfig(chat, cfg)
}

func isAllowedGroupChatWithConfig(chat *tele.Chat, config runtimeConfig) bool {
	if !isGroupChat(chat) {
		return false
	}
	if !isPublicGroupChat(chat) {
		return false
	}
	if config.isPublicMode() {
		return true
	}

	groupID := normalizePublicGroupLookupID(chat.Username)
	if groupID == "" {
		return false
	}
	_, ok := config.groupAllow[groupID]
	return ok
}

func isAllowedCommandChat(chat *tele.Chat) bool {
	return isAllowedCommandChatWithConfig(chat, cfg)
}

func isAllowedCommandChatWithConfig(chat *tele.Chat, config runtimeConfig) bool {
	if chat == nil {
		return false
	}
	if chat.Type == tele.ChatPrivate {
		return true
	}
	if isGroupChat(chat) {
		return isAllowedGroupChatWithConfig(chat, config)
	}
	return false
}

func leaveChat(chat *tele.Chat, reason string) {
	if chat == nil {
		return
	}
	if bot == nil {
		log.Printf("warn: leave skipped chat_id=%d reason=%s err=bot_not_initialized", chat.ID, reason)
		return
	}
	if err := bot.Leave(chat); err != nil {
		log.Printf("warn: failed to leave chat chat_id=%d reason=%s err=%v", chat.ID, reason, err)
	}
}

func leaveIfUnsupportedPrivateGroup(chat *tele.Chat, trigger string) bool {
	if !isGroupChat(chat) || isPublicGroupChat(chat) {
		return false
	}
	log.Printf("Unsupported chat type for captcha bot chat_id=%d chat_type=%s trigger=%s reason=private_group_without_username", chat.ID, chat.Type, trigger)
	leaveChat(chat, "private_group_without_username")
	return true
}

func isContextAuthorized(c tele.Context) bool {
	if c == nil || c.Chat() == nil {
		return false
	}
	return isAuthorizedGroupChatWithConfig(c.Chat(), cfg)
}

func isAuthorizedGroupChatWithConfig(chat *tele.Chat, config runtimeConfig) bool {
	if !isGroupChat(chat) {
		return false
	}
	return isAllowedGroupChatWithConfig(chat, config)
}

func isAllowedUserID(userID int64) bool {
	if userID <= 0 {
		return false
	}
	_, ok := cfg.Bot.adminUsers[userID]
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
	log.Printf("Access denied event=%s chat_id=%d user_id=%d public_mode=%t", event, chatID, userID, cfg.isPublicMode())
}

func onAddedToGroup(c tele.Context) error {
	if c == nil || c.Chat() == nil {
		return nil
	}
	if leaveIfUnsupportedPrivateGroup(c.Chat(), "added_to_group") {
		return nil
	}

	if isContextAuthorized(c) {
		return nil
	}
	logAccessDenied(c, "added_to_group")
	leaveChat(c.Chat(), "unauthorized_group")
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
