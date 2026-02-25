package app

import (
	"fmt"
	"log"
	"sort"
	"strings"

	tele "gopkg.in/telebot.v3"
	"toshiki-captcha-bot/internal/commandscope"
	"toshiki-captcha-bot/internal/settings"
	"toshiki-captcha-bot/internal/version"
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
		"/ping check bot reachability and latency in ms (admin ids only)",
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
	if c == nil || c.Chat() == nil {
		log.Printf("warn: help skipped reason=missing_chat_context user_id=%d", userID)
		return nil
	}
	if leaveIfUnsupportedPrivateGroup(c.Chat(), "help") {
		return nil
	}
	if !isAllowedCommandChat(c.Chat()) {
		logAccessDenied(c, "help_chat_not_allowed")
		if isGroupChat(c.Chat()) {
			leaveChat(c.Chat(), "unauthorized_group")
		}
		return nil
	}
	if _, err := sendWithConfiguredTopic(c.Chat(), helpText(), tele.ModeDefault, nil); err != nil {
		log.Printf("warn: failed to send help response chat_id=%d user_id=%d err=%v", chatID, userID, err)
	}
	return nil
}

func onVersion(c tele.Context) error {
	chatID, userID := commandContextIDs(c)
	log.Printf("Version requested chat_id=%d user_id=%d", chatID, userID)
	if c == nil || c.Chat() == nil {
		log.Printf("warn: version skipped reason=missing_chat_context user_id=%d", userID)
		return nil
	}
	if leaveIfUnsupportedPrivateGroup(c.Chat(), "version") {
		return nil
	}
	if !isAllowedCommandChat(c.Chat()) {
		logAccessDenied(c, "version_chat_not_allowed")
		if isGroupChat(c.Chat()) {
			leaveChat(c.Chat(), "unauthorized_group")
		}
		return nil
	}
	if _, err := sendWithConfiguredTopic(c.Chat(), version.MarkdownText(), tele.ModeMarkdown, nil); err != nil {
		log.Printf("warn: failed to send version response chat_id=%d user_id=%d err=%v", chatID, userID, err)
	}
	return nil
}

func syncBotCommands(b *tele.Bot) {
	if b == nil {
		return
	}
	clearLegacyAdminCommandScopes(b)

	public := publicBotCommands()
	if err := b.SetCommands(public); err != nil {
		log.Printf("warn: failed to register default bot commands err=%v", err)
	} else {
		log.Printf("Bot commands updated scope=default count=%d", len(public))
	}

	adminCommands := adminBotCommands()
	desiredScopes := desiredAdminCommandScopes(b, cfg)
	reconcileAdminCommandScopes(b, adminCommands, desiredScopes)
	if len(desiredScopes) == 0 {
		log.Printf("Bot commands admin scopes skipped reason=no_admin_user_ids")
	}
}

func clearLegacyAdminCommandScopes(b *tele.Bot) {
	if b == nil {
		return
	}
	legacyScope := tele.CommandScope{Type: tele.CommandScopeAllChatAdmin}
	if err := b.DeleteCommands(legacyScope); err != nil {
		log.Printf("warn: failed to delete legacy admin command scope scope=%s err=%v", legacyScope.Type, err)
		return
	}
	log.Printf("Bot commands deleted legacy scope=%s", legacyScope.Type)
}

func desiredAdminCommandScopes(b *tele.Bot, config settings.RuntimeConfig) []tele.CommandScope {
	adminIDs := sortedAdminUserIDs(config)
	if len(adminIDs) == 0 {
		return nil
	}

	groupChatIDs := []int64{}
	if !config.IsPublicMode() {
		groupChatIDs = resolveConfiguredGroupChatIDs(b, config.GroupsList())
	}

	return buildAdminCommandScopes(adminIDs, groupChatIDs)
}

func reconcileAdminCommandScopes(b *tele.Bot, adminCommands []tele.Command, desiredScopes []tele.CommandScope) {
	if b == nil {
		return
	}

	previousScopes, err := commandscope.Load(commandScopeStatePath)
	if err != nil {
		log.Printf("warn: failed to load command scope state path=%q err=%v", commandScopeStatePath, err)
	}

	staleScopes := commandscope.DiffScopes(previousScopes, desiredScopes)
	failedDeletes := make([]tele.CommandScope, 0)
	for _, scope := range staleScopes {
		if err := b.DeleteCommands(scope); err != nil {
			log.Printf(
				"warn: failed to delete stale admin bot command scope scope=%s chat_id=%d user_id=%d err=%v",
				scope.Type,
				scope.ChatID,
				scope.UserID,
				err,
			)
			failedDeletes = append(failedDeletes, scope)
			continue
		}
		log.Printf("Bot commands deleted stale scope=%s chat_id=%d user_id=%d", scope.Type, scope.ChatID, scope.UserID)
	}

	success := 0
	for _, scope := range desiredScopes {
		if err := b.SetCommands(adminCommands, scope); err != nil {
			log.Printf(
				"warn: failed to register admin bot commands scope=%s chat_id=%d user_id=%d err=%v",
				scope.Type,
				scope.ChatID,
				scope.UserID,
				err,
			)
			continue
		}
		success++
	}
	log.Printf("Bot commands updated admin_scopes=%d success=%d stale_deleted=%d", len(desiredScopes), success, len(staleScopes)-len(failedDeletes))

	nextState := commandscope.MergeScopes(desiredScopes, failedDeletes)
	if err := commandscope.Save(commandScopeStatePath, nextState); err != nil {
		log.Printf("warn: failed to save command scope state path=%q err=%v", commandScopeStatePath, err)
		return
	}
	log.Printf("Bot commands state saved path=%q scopes=%d", commandScopeStatePath, len(nextState))
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

func sortedAdminUserIDs(config settings.RuntimeConfig) []int64 {
	ids := append([]int64(nil), config.Bot.AdminUserIDs...)
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})
	return ids
}

func resolveConfiguredGroupChatIDs(b *tele.Bot, groups []settings.GroupTopicConfig) []int64 {
	ids := make([]int64, 0, len(groups))
	seen := make(map[int64]struct{}, len(groups))
	for _, group := range groups {
		chat, err := b.ChatByUsername(group.ID)
		if err != nil {
			log.Printf("warn: failed to resolve group chat_id by username group=%s err=%v", group.ID, err)
			continue
		}
		if chat == nil {
			log.Printf("warn: resolved group is nil group=%s", group.ID)
			continue
		}
		if !isPublicGroupChat(chat) {
			log.Printf("warn: resolved group is not supported for admin command scope group=%s chat_id=%d chat_type=%s", group.ID, chat.ID, chat.Type)
			continue
		}
		if _, ok := seen[chat.ID]; ok {
			continue
		}
		seen[chat.ID] = struct{}{}
		ids = append(ids, chat.ID)
	}
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})
	return ids
}

func buildAdminCommandScopes(adminUserIDs []int64, groupChatIDs []int64) []tele.CommandScope {
	scopes := make([]tele.CommandScope, 0, len(adminUserIDs)*(len(groupChatIDs)+1))
	seen := make(map[string]struct{}, len(adminUserIDs)*(len(groupChatIDs)+1))
	for _, userID := range adminUserIDs {
		scope := tele.CommandScope{
			Type:   tele.CommandScopeChat,
			ChatID: userID,
		}
		key := fmt.Sprintf("%s:%d:%d", scope.Type, scope.ChatID, scope.UserID)
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			scopes = append(scopes, scope)
		}

		for _, groupChatID := range groupChatIDs {
			scope = tele.CommandScope{
				Type:   tele.CommandScopeChatMember,
				ChatID: groupChatID,
				UserID: userID,
			}
			key = fmt.Sprintf("%s:%d:%d", scope.Type, scope.ChatID, scope.UserID)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			scopes = append(scopes, scope)
		}
	}
	return scopes
}

func adminOnlyCommandErrorText(command string) string {
	return fmt.Sprintf("Access denied: %s is available only to configured admin user IDs.", command)
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
