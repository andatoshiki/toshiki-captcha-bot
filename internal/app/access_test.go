package app

import (
	"testing"

	tele "gopkg.in/telebot.v3"
)

func TestIsGroupChat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		chat *tele.Chat
		want bool
	}{
		{
			name: "nil chat",
			chat: nil,
			want: false,
		},
		{
			name: "private user chat",
			chat: &tele.Chat{Type: tele.ChatPrivate},
			want: false,
		},
		{
			name: "group chat",
			chat: &tele.Chat{Type: tele.ChatGroup},
			want: true,
		},
		{
			name: "supergroup chat",
			chat: &tele.Chat{Type: tele.ChatSuperGroup},
			want: true,
		},
		{
			name: "channel chat",
			chat: &tele.Chat{Type: tele.ChatChannel},
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isGroupChat(tt.chat); got != tt.want {
				t.Fatalf("isGroupChat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsPublicGroupChat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		chat *tele.Chat
		want bool
	}{
		{
			name: "nil chat",
			chat: nil,
			want: false,
		},
		{
			name: "private user chat",
			chat: &tele.Chat{Type: tele.ChatPrivate, Username: "anything"},
			want: false,
		},
		{
			name: "group with username",
			chat: &tele.Chat{Type: tele.ChatGroup, Username: "mygroup"},
			want: true,
		},
		{
			name: "supergroup with username",
			chat: &tele.Chat{Type: tele.ChatSuperGroup, Username: "mysupergroup"},
			want: true,
		},
		{
			name: "private group without username",
			chat: &tele.Chat{Type: tele.ChatSuperGroup, Username: ""},
			want: false,
		},
		{
			name: "private group with whitespace username",
			chat: &tele.Chat{Type: tele.ChatSuperGroup, Username: "  "},
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isPublicGroupChat(tt.chat); got != tt.want {
				t.Fatalf("isPublicGroupChat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsAllowedGroupChatWithConfig(t *testing.T) {
	t.Parallel()

	publicCfg := runtimeConfig{
		Bot: botConfig{
			adminUsers: map[int64]struct{}{},
		},
	}
	privateCfg := runtimeConfig{
		Bot: botConfig{
			adminUsers: map[int64]struct{}{1001: {}},
		},
		groupAllow: map[string]struct{}{
			"allowedgroup": {},
		},
	}

	tests := []struct {
		name   string
		cfg    runtimeConfig
		chat   *tele.Chat
		wantOK bool
	}{
		{
			name:   "public mode allows any public group",
			cfg:    publicCfg,
			chat:   &tele.Chat{Type: tele.ChatSuperGroup, Username: "anygroup"},
			wantOK: true,
		},
		{
			name:   "public mode rejects private groups",
			cfg:    publicCfg,
			chat:   &tele.Chat{Type: tele.ChatSuperGroup, Username: ""},
			wantOK: false,
		},
		{
			name:   "private mode allows only listed groups",
			cfg:    privateCfg,
			chat:   &tele.Chat{Type: tele.ChatSuperGroup, Username: "allowedgroup"},
			wantOK: true,
		},
		{
			name:   "private mode rejects unlisted groups",
			cfg:    privateCfg,
			chat:   &tele.Chat{Type: tele.ChatSuperGroup, Username: "othergroup"},
			wantOK: false,
		},
		{
			name:   "private mode rejects private groups without username",
			cfg:    privateCfg,
			chat:   &tele.Chat{Type: tele.ChatSuperGroup, Username: ""},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isAllowedGroupChatWithConfig(tt.chat, tt.cfg); got != tt.wantOK {
				t.Fatalf("isAllowedGroupChatWithConfig() = %v, want %v", got, tt.wantOK)
			}
		})
	}
}

func TestIsAuthorizedGroupChatWithConfig(t *testing.T) {
	t.Parallel()

	cfg := runtimeConfig{
		Bot: botConfig{
			adminUsers: map[int64]struct{}{1001: {}},
		},
		groupAllow: map[string]struct{}{
			"allowedgroup": {},
		},
	}

	tests := []struct {
		name   string
		chat   *tele.Chat
		wantOK bool
	}{
		{
			name:   "allowed group in private mode",
			chat:   &tele.Chat{Type: tele.ChatSuperGroup, Username: "allowedgroup"},
			wantOK: true,
		},
		{
			name:   "unlisted group in private mode",
			chat:   &tele.Chat{Type: tele.ChatSuperGroup, Username: "othergroup"},
			wantOK: false,
		},
		{
			name:   "private user chat is not group authorized",
			chat:   &tele.Chat{Type: tele.ChatPrivate},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isAuthorizedGroupChatWithConfig(tt.chat, cfg); got != tt.wantOK {
				t.Fatalf("isAuthorizedGroupChatWithConfig() = %v, want %v", got, tt.wantOK)
			}
		})
	}
}

func TestIsAllowedCommandChatWithConfig(t *testing.T) {
	t.Parallel()

	privateCfg := runtimeConfig{
		Bot: botConfig{
			adminUsers: map[int64]struct{}{1001: {}},
		},
		groupAllow: map[string]struct{}{
			"allowedgroup": {},
		},
	}
	publicCfg := runtimeConfig{
		Bot: botConfig{
			adminUsers: map[int64]struct{}{},
		},
	}

	tests := []struct {
		name   string
		cfg    runtimeConfig
		chat   *tele.Chat
		wantOK bool
	}{
		{
			name:   "private user chat allowed",
			cfg:    privateCfg,
			chat:   &tele.Chat{Type: tele.ChatPrivate},
			wantOK: true,
		},
		{
			name:   "allowlisted public group allowed",
			cfg:    privateCfg,
			chat:   &tele.Chat{Type: tele.ChatSuperGroup, Username: "allowedgroup"},
			wantOK: true,
		},
		{
			name:   "unlisted public group rejected",
			cfg:    privateCfg,
			chat:   &tele.Chat{Type: tele.ChatSuperGroup, Username: "othergroup"},
			wantOK: false,
		},
		{
			name:   "private group rejected",
			cfg:    privateCfg,
			chat:   &tele.Chat{Type: tele.ChatSuperGroup, Username: ""},
			wantOK: false,
		},
		{
			name:   "channel rejected",
			cfg:    privateCfg,
			chat:   &tele.Chat{Type: tele.ChatChannel, Username: "chan"},
			wantOK: false,
		},
		{
			name:   "public mode allows command in any public group",
			cfg:    publicCfg,
			chat:   &tele.Chat{Type: tele.ChatSuperGroup, Username: "somepublicgroup"},
			wantOK: true,
		},
		{
			name:   "public mode still rejects private group without username",
			cfg:    publicCfg,
			chat:   &tele.Chat{Type: tele.ChatSuperGroup, Username: ""},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isAllowedCommandChatWithConfig(tt.chat, tt.cfg); got != tt.wantOK {
				t.Fatalf("isAllowedCommandChatWithConfig() = %v, want %v", got, tt.wantOK)
			}
		})
	}
}
