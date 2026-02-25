package app

import (
	"errors"
	"strings"
	"testing"

	tele "gopkg.in/telebot.v3"
	"toshiki-captcha-bot/internal/settings"
)

func mustValidatedRuntimeConfig(t *testing.T, cfg settings.RuntimeConfig) settings.RuntimeConfig {
	t.Helper()
	cfg.Bot.Token = "test-token"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	return cfg
}

func TestBuildSendOptionsWithTopic(t *testing.T) {
	t.Parallel()

	markup := &tele.ReplyMarkup{}

	tests := []struct {
		name     string
		topicID  int
		wantID   int
		wantMode tele.ParseMode
	}{
		{
			name:     "without topic",
			topicID:  0,
			wantID:   0,
			wantMode: tele.ModeDefault,
		},
		{
			name:     "with topic and markdown",
			topicID:  9,
			wantID:   9,
			wantMode: tele.ModeMarkdown,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := buildSendOptionsWithTopic(tt.wantMode, markup, tt.topicID)
			if opts == nil {
				t.Fatalf("buildSendOptionsWithTopic returned nil")
			}
			if opts.ParseMode != tt.wantMode {
				t.Fatalf("ParseMode = %q, want %q", opts.ParseMode, tt.wantMode)
			}
			if opts.ReplyMarkup != markup {
				t.Fatalf("ReplyMarkup pointer mismatch")
			}
			if opts.ThreadID != tt.wantID {
				t.Fatalf("ThreadID = %d, want %d", opts.ThreadID, tt.wantID)
			}
		})
	}
}

func TestTopicThreadIDForChat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		cfg    settings.RuntimeConfig
		chat   *tele.Chat
		wantID int
	}{
		{
			name:   "nil chat falls back to root",
			cfg:    settings.RuntimeConfig{},
			chat:   nil,
			wantID: 0,
		},
		{
			name:   "private chat ignores configured topic",
			cfg:    settings.RuntimeConfig{},
			chat:   &tele.Chat{Type: tele.ChatPrivate},
			wantID: 0,
		},
		{
			name:   "public mode discards groups topic mapping",
			cfg:    mustValidatedRuntimeConfig(t, settings.DefaultRuntimeConfig()),
			chat:   &tele.Chat{Type: tele.ChatSuperGroup, Username: "somegroup"},
			wantID: 0,
		},
		{
			name: "private mode resolves configured topic",
			cfg: func(t *testing.T) settings.RuntimeConfig {
				cfg := settings.DefaultRuntimeConfig()
				cfg.Bot.AdminUserIDs = []int64{1001}
				cfg.Groups = []settings.GroupTopicConfig{{ID: "@somegroup", Topic: 42}}
				return mustValidatedRuntimeConfig(t, cfg)
			}(t),
			chat:   &tele.Chat{Type: tele.ChatSuperGroup, Username: "somegroup"},
			wantID: 42,
		},
		{
			name: "private mode treats topic one as root",
			cfg: func(t *testing.T) settings.RuntimeConfig {
				cfg := settings.DefaultRuntimeConfig()
				cfg.Bot.AdminUserIDs = []int64{1001}
				cfg.Groups = []settings.GroupTopicConfig{{ID: "@somegroup", Topic: 1}}
				return mustValidatedRuntimeConfig(t, cfg)
			}(t),
			chat:   &tele.Chat{Type: tele.ChatSuperGroup, Username: "somegroup"},
			wantID: 0,
		},
		{
			name: "private mode returns root for unknown group",
			cfg: func(t *testing.T) settings.RuntimeConfig {
				cfg := settings.DefaultRuntimeConfig()
				cfg.Bot.AdminUserIDs = []int64{1001}
				cfg.Groups = []settings.GroupTopicConfig{{ID: "@somegroup", Topic: 42}}
				return mustValidatedRuntimeConfig(t, cfg)
			}(t),
			chat:   &tele.Chat{Type: tele.ChatSuperGroup, Username: "othergroup"},
			wantID: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := resolveTopicThreadIDForChat(tt.chat, tt.cfg); got != tt.wantID {
				t.Fatalf("resolveTopicThreadIDForChat() = %d, want %d", got, tt.wantID)
			}
		})
	}
}

func TestEscapeTelegramMarkdown(t *testing.T) {
	t.Parallel()

	input := `a_b*c[d]e\` + "`" + `f`
	got := escapeTelegramMarkdown(input)
	want := `a\_b\*c\[d\]e\\\` + "`" + `f`
	if got != want {
		t.Fatalf("escapeTelegramMarkdown() = %q, want %q", got, want)
	}
}

func TestGenCaptionEscapesDisplayName(t *testing.T) {
	oldCfg := cfg
	cfg = mustValidatedRuntimeConfig(t, settings.DefaultRuntimeConfig())
	t.Cleanup(func() {
		cfg = oldCfg
	})

	user := &tele.User{
		ID:        1234,
		FirstName: "a_b*[x]",
	}
	caption := genCaption(user)
	if !strings.Contains(caption, `[a\_b\*\[x\]](tg://user?id=1234)`) {
		t.Fatalf("caption mention is not escaped correctly: %q", caption)
	}
}

func resetManagedMessagesIndexForTest() {
	managedMessagesIndex.mu.Lock()
	managedMessagesIndex.byChat = make(map[int64]map[int]struct{})
	managedMessagesIndex.mu.Unlock()
}

func TestManagedMessageIndexTracking(t *testing.T) {
	resetManagedMessagesIndexForTest()

	msg := &tele.Message{
		ID:   10,
		Chat: &tele.Chat{ID: -1001},
	}
	registerManagedBotMessage(msg)

	ids := managedMessageIDsForChat(-1001)
	if len(ids) != 1 || ids[0] != 10 {
		t.Fatalf("managed ids = %v, want [10]", ids)
	}

	if ok := hasManagedBotMessage(-1001, 10); !ok {
		t.Fatalf("hasManagedBotMessage = false, want true")
	}
	if ok := removeManagedBotMessage(-1001, 10); !ok {
		t.Fatalf("removeManagedBotMessage first call = false, want true")
	}
	if ok := removeManagedBotMessage(-1001, 10); ok {
		t.Fatalf("removeManagedBotMessage second call = true, want false")
	}
	if got := managedMessageIDsForChat(-1001); len(got) != 0 {
		t.Fatalf("managed ids after consume = %v, want empty", got)
	}
}

func TestClearManagedBotMessagesInChatWithoutBot(t *testing.T) {
	resetManagedMessagesIndexForTest()

	oldBot := bot
	bot = nil
	t.Cleanup(func() {
		bot = oldBot
		resetManagedMessagesIndexForTest()
	})

	registerManagedBotMessage(&tele.Message{ID: 11, Chat: &tele.Chat{ID: -1001}})
	registerManagedBotMessage(&tele.Message{ID: 12, Chat: &tele.Chat{ID: -1001}})
	registerManagedBotMessage(&tele.Message{ID: 21, Chat: &tele.Chat{ID: -1002}})

	deleted, failed := clearManagedBotMessagesInChat(&tele.Chat{ID: -1001})
	if deleted != 0 {
		t.Fatalf("deleted = %d, want 0 when bot is nil", deleted)
	}
	if failed != 2 {
		t.Fatalf("failed = %d, want 2 when bot is nil", failed)
	}

	if got := managedMessageIDsForChat(-1001); len(got) != 2 {
		t.Fatalf("managed ids for chat -1001 = %v, want len 2", got)
	}
	if got := managedMessageIDsForChat(-1002); len(got) != 1 || got[0] != 21 {
		t.Fatalf("managed ids for chat -1002 = %v, want [21]", got)
	}
}

func TestClearManagedBotMessagesInChatTreatsNotFoundAsDeleted(t *testing.T) {
	resetManagedMessagesIndexForTest()

	oldBot := bot
	oldDeleteFn := deleteManagedBotMessageFn
	bot = &tele.Bot{}
	deleteManagedBotMessageFn = func(_ tele.Editable) error {
		return errors.New("telegram: Bad Request: message to delete not found (400)")
	}
	t.Cleanup(func() {
		bot = oldBot
		deleteManagedBotMessageFn = oldDeleteFn
		resetManagedMessagesIndexForTest()
	})

	chat := &tele.Chat{ID: -1001}
	registerManagedBotMessage(&tele.Message{ID: 31, Chat: chat})

	deleted, failed := clearManagedBotMessagesInChat(chat)
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1 for already-deleted message", deleted)
	}
	if failed != 0 {
		t.Fatalf("failed = %d, want 0 for already-deleted message", failed)
	}

	if got := managedMessageIDsForChat(-1001); len(got) != 0 {
		t.Fatalf("managed ids for chat -1001 = %v, want empty", got)
	}
}
