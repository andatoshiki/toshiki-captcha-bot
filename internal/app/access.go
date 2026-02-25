package app

import (
	"fmt"
	"log"

	tele "gopkg.in/telebot.v3"
	"toshiki-captcha-bot/internal/captcha"
	"toshiki-captcha-bot/internal/policy"
)

func isGroupChat(chat *tele.Chat) bool {
	return policy.IsGroupChat(chat)
}

func isPublicGroupChat(chat *tele.Chat) bool {
	return policy.IsPublicGroupChat(chat)
}

func isAllowedCommandChat(chat *tele.Chat) bool {
	return policy.IsAllowedCommandChat(chat, cfg)
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
	if !policy.IsGroupChat(chat) || policy.IsPublicGroupChat(chat) {
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
	return policy.IsAuthorizedGroupChat(c.Chat(), cfg)
}

func isSenderAllowed(c tele.Context) bool {
	if c == nil || c.Sender() == nil {
		return false
	}
	return policy.IsAllowedUserID(c.Sender().ID, cfg)
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
	log.Printf("Access denied event=%s chat_id=%d user_id=%d public_mode=%t", event, chatID, userID, cfg.IsPublicMode())
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
		cleanupPendingCaptchaForUser(c.Chat(), c.Sender())
		log.Printf("User left user_id=%d chat_id=%d", c.Sender().ID, c.Chat().ID)
	}

	return nil
}

func cleanupPendingCaptchaForUser(chat *tele.Chat, user *tele.User) {
	if chat == nil || user == nil || db == nil {
		return
	}

	kvID := fmt.Sprintf("%v-%v", user.ID, chat.ID)
	value, found := db.Get(kvID)
	if !found {
		return
	}

	if status, ok := value.(captcha.JoinStatus); ok {
		if bot == nil {
			log.Printf("warn: pending captcha cleanup skipped reason=bot_not_initialized chat_id=%d user_id=%d", chat.ID, user.ID)
		} else if status.CaptchaMessage.ID > 0 {
			if err := bot.Delete(&status.CaptchaMessage); err != nil {
				log.Printf("warn: failed to delete pending captcha on user leave chat_id=%d user_id=%d message_id=%d err=%v", chat.ID, user.ID, status.CaptchaMessage.ID, err)
			}
		}
	}

	if err := db.Delete(kvID); err != nil {
		log.Printf("warn: failed to delete pending captcha state on user leave chat_id=%d user_id=%d err=%v", chat.ID, user.ID, err)
	}
}
