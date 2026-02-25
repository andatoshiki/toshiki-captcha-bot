package app

import (
	"reflect"
	"strings"
	"testing"

	tele "gopkg.in/telebot.v3"
)

func TestHelpText(t *testing.T) {
	t.Parallel()

	got := helpText()

	required := []string{
		"This bot protects group joins with an emoji captcha",
		"/help",
		"/version",
		"/ping",
		"/testcaptcha",
		"admin ids only",
		projectURL,
		authorInfo,
	}

	for _, entry := range required {
		if !strings.Contains(got, entry) {
			t.Fatalf("helpText missing %q", entry)
		}
	}
}

func TestPublicBotCommands(t *testing.T) {
	t.Parallel()

	cmds := publicBotCommands()
	if len(cmds) != 2 {
		t.Fatalf("public command count = %d, want 2", len(cmds))
	}

	got := []string{cmds[0].Text, cmds[1].Text}
	want := []string{"help", "version"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("public commands = %v, want %v", got, want)
	}
}

func TestAdminBotCommands(t *testing.T) {
	t.Parallel()

	cmds := adminBotCommands()
	if len(cmds) != 4 {
		t.Fatalf("admin command count = %d, want 4", len(cmds))
	}

	got := []string{cmds[0].Text, cmds[1].Text, cmds[2].Text, cmds[3].Text}
	want := []string{"help", "version", "ping", "testcaptcha"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("admin commands = %v, want %v", got, want)
	}
}

func TestBuildAdminCommandScopes(t *testing.T) {
	t.Parallel()

	adminIDs := []int64{1002, 1001}
	groupChatIDs := []int64{-1001234567890, -1001111111111}

	got := buildAdminCommandScopes(adminIDs, groupChatIDs)

	want := []tele.CommandScope{
		{Type: tele.CommandScopeChat, ChatID: 1002},
		{Type: tele.CommandScopeChatMember, ChatID: -1001234567890, UserID: 1002},
		{Type: tele.CommandScopeChatMember, ChatID: -1001111111111, UserID: 1002},
		{Type: tele.CommandScopeChat, ChatID: 1001},
		{Type: tele.CommandScopeChatMember, ChatID: -1001234567890, UserID: 1001},
		{Type: tele.CommandScopeChatMember, ChatID: -1001111111111, UserID: 1001},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildAdminCommandScopes() = %#v, want %#v", got, want)
	}
}

func TestBuildAdminCommandScopesDedup(t *testing.T) {
	t.Parallel()

	adminIDs := []int64{1001, 1001}
	groupChatIDs := []int64{-1001, -1001}

	got := buildAdminCommandScopes(adminIDs, groupChatIDs)

	want := []tele.CommandScope{
		{Type: tele.CommandScopeChat, ChatID: 1001},
		{Type: tele.CommandScopeChatMember, ChatID: -1001, UserID: 1001},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildAdminCommandScopes() = %#v, want %#v", got, want)
	}
}

func TestSortedAdminUserIDs(t *testing.T) {
	t.Parallel()

	config := runtimeConfig{
		Bot: botConfig{
			adminUsers: map[int64]struct{}{
				4002: {},
				1001: {},
				3003: {},
			},
		},
	}

	got := sortedAdminUserIDs(config)
	want := []int64{1001, 3003, 4002}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sortedAdminUserIDs() = %v, want %v", got, want)
	}
}

func TestAdminOnlyCommandErrorText(t *testing.T) {
	t.Parallel()

	got := adminOnlyCommandErrorText("/ping")
	if !strings.Contains(got, "/ping") {
		t.Fatalf("adminOnlyCommandErrorText missing command: %q", got)
	}
	if !strings.Contains(got, "Access denied:") {
		t.Fatalf("adminOnlyCommandErrorText missing prefix: %q", got)
	}
}
