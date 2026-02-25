package app

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	tele "gopkg.in/telebot.v3"
	"toshiki-captcha-bot/internal/settings"
)

var managedMessagesIndex = struct {
	mu     sync.Mutex
	byChat map[int64]map[int]struct{}
}{
	byChat: make(map[int64]map[int]struct{}),
}

var deleteManagedBotMessageFn = func(msg tele.Editable) error {
	return bot.Delete(msg)
}

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

func scheduleBotMessageCleanup(msg *tele.Message, reason string) {
	if msg == nil || msg.ID == 0 || bot == nil {
		return
	}
	registerManagedBotMessage(msg)

	ttl := cfg.Bot.MessageCleanup
	if ttl <= 0 {
		return
	}

	message := *msg
	chatID := int64(0)
	if message.Chat != nil {
		chatID = message.Chat.ID
	}

	go func() {
		time.Sleep(ttl)
		if !takeManagedBotMessage(chatID, message.ID) {
			return
		}
		if err := deleteManagedBotMessageFn(&message); err != nil {
			if isMessageAlreadyDeletedError(err) {
				return
			}
			log.Printf(
				"warn: failed to auto delete bot message chat_id=%d message_id=%d reason=%s ttl=%s err=%v",
				chatID,
				message.ID,
				reason,
				ttl,
				err,
			)
			registerManagedBotMessage(&message)
			return
		}
	}()
}

func registerManagedBotMessage(msg *tele.Message) {
	if msg == nil || msg.Chat == nil || msg.ID == 0 {
		return
	}
	managedMessagesIndex.mu.Lock()
	defer managedMessagesIndex.mu.Unlock()

	chatIndex, ok := managedMessagesIndex.byChat[msg.Chat.ID]
	if !ok {
		chatIndex = make(map[int]struct{})
		managedMessagesIndex.byChat[msg.Chat.ID] = chatIndex
	}
	chatIndex[msg.ID] = struct{}{}
}

func hasManagedBotMessage(chatID int64, messageID int) bool {
	if messageID == 0 {
		return false
	}

	managedMessagesIndex.mu.Lock()
	defer managedMessagesIndex.mu.Unlock()

	chatIndex, ok := managedMessagesIndex.byChat[chatID]
	if !ok {
		return false
	}
	_, exists := chatIndex[messageID]
	return exists
}

func takeManagedBotMessage(chatID int64, messageID int) bool {
	if messageID == 0 {
		return false
	}

	managedMessagesIndex.mu.Lock()
	defer managedMessagesIndex.mu.Unlock()

	chatIndex, ok := managedMessagesIndex.byChat[chatID]
	if !ok {
		return false
	}
	if _, exists := chatIndex[messageID]; !exists {
		return false
	}
	delete(chatIndex, messageID)
	if len(chatIndex) == 0 {
		delete(managedMessagesIndex.byChat, chatID)
	}
	return true
}

func removeManagedBotMessage(chatID int64, messageID int) bool {
	return takeManagedBotMessage(chatID, messageID)
}

func managedMessageIDsForChat(chatID int64) []int {
	managedMessagesIndex.mu.Lock()
	defer managedMessagesIndex.mu.Unlock()

	chatIndex, ok := managedMessagesIndex.byChat[chatID]
	if !ok || len(chatIndex) == 0 {
		return nil
	}

	ids := make([]int, 0, len(chatIndex))
	for id := range chatIndex {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func clearManagedBotMessagesInChat(chat *tele.Chat) (deleted int, failed int) {
	if chat == nil {
		return 0, 0
	}

	ids := managedMessageIDsForChat(chat.ID)
	if len(ids) == 0 {
		return 0, 0
	}

	for _, messageID := range ids {
		if !takeManagedBotMessage(chat.ID, messageID) {
			continue
		}
		if bot == nil {
			failed++
			registerManagedBotMessage(&tele.Message{ID: messageID, Chat: chat})
			continue
		}
		msg := &tele.Message{ID: messageID, Chat: chat}
		if err := deleteManagedBotMessageFn(msg); err != nil {
			if isMessageAlreadyDeletedError(err) {
				deleted++
				continue
			}
			failed++
			log.Printf("warn: failed to clear managed bot message chat_id=%d message_id=%d err=%v", chat.ID, messageID, err)
			registerManagedBotMessage(msg)
			continue
		}
		deleted++
	}

	return deleted, failed
}

func isMessageAlreadyDeletedError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "message to delete not found") ||
		strings.Contains(lower, "message not found")
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
