package policy

import (
	"testing"

	tele "gopkg.in/telebot.v3"
	"toshiki-captcha-bot/internal/settings"
)

func mustValidateConfig(t *testing.T, cfg settings.RuntimeConfig) settings.RuntimeConfig {
	t.Helper()
	cfg.Bot.Token = "test-token"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	return cfg
}

func TestIsGroupChat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		chat *tele.Chat
		want bool
	}{
		{name: "nil chat", chat: nil, want: false},
		{name: "private user chat", chat: &tele.Chat{Type: tele.ChatPrivate}, want: false},
		{name: "group chat", chat: &tele.Chat{Type: tele.ChatGroup}, want: true},
		{name: "supergroup chat", chat: &tele.Chat{Type: tele.ChatSuperGroup}, want: true},
		{name: "channel chat", chat: &tele.Chat{Type: tele.ChatChannel}, want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsGroupChat(tt.chat); got != tt.want {
				t.Fatalf("IsGroupChat() = %v, want %v", got, tt.want)
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
		{name: "nil chat", chat: nil, want: false},
		{name: "private user chat", chat: &tele.Chat{Type: tele.ChatPrivate, Username: "anything"}, want: false},
		{name: "group with username", chat: &tele.Chat{Type: tele.ChatGroup, Username: "mygroup"}, want: true},
		{name: "supergroup with username", chat: &tele.Chat{Type: tele.ChatSuperGroup, Username: "mysupergroup"}, want: true},
		{name: "private group without username", chat: &tele.Chat{Type: tele.ChatSuperGroup, Username: ""}, want: false},
		{name: "private group with whitespace username", chat: &tele.Chat{Type: tele.ChatSuperGroup, Username: "  "}, want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsPublicGroupChat(tt.chat); got != tt.want {
				t.Fatalf("IsPublicGroupChat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsAllowedGroupChat(t *testing.T) {
	t.Parallel()

	publicCfg := mustValidateConfig(t, settings.DefaultRuntimeConfig())
	privateCfg := settings.DefaultRuntimeConfig()
	privateCfg.Bot.AdminUserIDs = []int64{1001}
	privateCfg.Groups = []settings.GroupTopicConfig{{ID: "@allowedgroup"}}
	privateCfg = mustValidateConfig(t, privateCfg)

	tests := []struct {
		name   string
		cfg    settings.RuntimeConfig
		chat   *tele.Chat
		wantOK bool
	}{
		{name: "public mode allows any public group", cfg: publicCfg, chat: &tele.Chat{Type: tele.ChatSuperGroup, Username: "anygroup"}, wantOK: true},
		{name: "public mode rejects private groups", cfg: publicCfg, chat: &tele.Chat{Type: tele.ChatSuperGroup, Username: ""}, wantOK: false},
		{name: "private mode allows only listed groups", cfg: privateCfg, chat: &tele.Chat{Type: tele.ChatSuperGroup, Username: "allowedgroup"}, wantOK: true},
		{name: "private mode rejects unlisted groups", cfg: privateCfg, chat: &tele.Chat{Type: tele.ChatSuperGroup, Username: "othergroup"}, wantOK: false},
		{name: "private mode rejects private groups without username", cfg: privateCfg, chat: &tele.Chat{Type: tele.ChatSuperGroup, Username: ""}, wantOK: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsAllowedGroupChat(tt.chat, tt.cfg); got != tt.wantOK {
				t.Fatalf("IsAllowedGroupChat() = %v, want %v", got, tt.wantOK)
			}
		})
	}
}

func TestIsAuthorizedGroupChat(t *testing.T) {
	t.Parallel()

	cfg := settings.DefaultRuntimeConfig()
	cfg.Bot.AdminUserIDs = []int64{1001}
	cfg.Groups = []settings.GroupTopicConfig{{ID: "@allowedgroup"}}
	cfg = mustValidateConfig(t, cfg)

	tests := []struct {
		name   string
		chat   *tele.Chat
		wantOK bool
	}{
		{name: "allowed group in private mode", chat: &tele.Chat{Type: tele.ChatSuperGroup, Username: "allowedgroup"}, wantOK: true},
		{name: "unlisted group in private mode", chat: &tele.Chat{Type: tele.ChatSuperGroup, Username: "othergroup"}, wantOK: false},
		{name: "private user chat is not group authorized", chat: &tele.Chat{Type: tele.ChatPrivate}, wantOK: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsAuthorizedGroupChat(tt.chat, cfg); got != tt.wantOK {
				t.Fatalf("IsAuthorizedGroupChat() = %v, want %v", got, tt.wantOK)
			}
		})
	}
}

func TestIsAllowedCommandChat(t *testing.T) {
	t.Parallel()

	privateCfg := settings.DefaultRuntimeConfig()
	privateCfg.Bot.AdminUserIDs = []int64{1001}
	privateCfg.Groups = []settings.GroupTopicConfig{{ID: "@allowedgroup"}}
	privateCfg = mustValidateConfig(t, privateCfg)

	publicCfg := mustValidateConfig(t, settings.DefaultRuntimeConfig())

	tests := []struct {
		name   string
		cfg    settings.RuntimeConfig
		chat   *tele.Chat
		wantOK bool
	}{
		{name: "private user chat allowed", cfg: privateCfg, chat: &tele.Chat{Type: tele.ChatPrivate}, wantOK: true},
		{name: "allowlisted public group allowed", cfg: privateCfg, chat: &tele.Chat{Type: tele.ChatSuperGroup, Username: "allowedgroup"}, wantOK: true},
		{name: "unlisted public group rejected", cfg: privateCfg, chat: &tele.Chat{Type: tele.ChatSuperGroup, Username: "othergroup"}, wantOK: false},
		{name: "private group rejected", cfg: privateCfg, chat: &tele.Chat{Type: tele.ChatSuperGroup, Username: ""}, wantOK: false},
		{name: "channel rejected", cfg: privateCfg, chat: &tele.Chat{Type: tele.ChatChannel, Username: "chan"}, wantOK: false},
		{name: "public mode allows command in any public group", cfg: publicCfg, chat: &tele.Chat{Type: tele.ChatSuperGroup, Username: "somepublicgroup"}, wantOK: true},
		{name: "public mode still rejects private group without username", cfg: publicCfg, chat: &tele.Chat{Type: tele.ChatSuperGroup, Username: ""}, wantOK: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsAllowedCommandChat(tt.chat, tt.cfg); got != tt.wantOK {
				t.Fatalf("IsAllowedCommandChat() = %v, want %v", got, tt.wantOK)
			}
		})
	}
}
