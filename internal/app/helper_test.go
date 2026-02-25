package app

import (
	"strings"
	"testing"

	tele "gopkg.in/telebot.v3"
)

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
		cfg    runtimeConfig
		chat   *tele.Chat
		wantID int
	}{
		{
			name:   "nil chat falls back to root",
			cfg:    runtimeConfig{},
			chat:   nil,
			wantID: 0,
		},
		{
			name:   "private chat ignores configured topic",
			cfg:    runtimeConfig{},
			chat:   &tele.Chat{Type: tele.ChatPrivate},
			wantID: 0,
		},
		{
			name: "public mode discards groups topic mapping",
			cfg: runtimeConfig{
				groupTopics: map[string]int{"somegroup": 42},
			},
			chat:   &tele.Chat{Type: tele.ChatSuperGroup, Username: "somegroup"},
			wantID: 0,
		},
		{
			name: "private mode resolves configured topic",
			cfg: runtimeConfig{
				Bot: botConfig{
					adminUsers: map[int64]struct{}{1001: {}},
				},
				groupTopics: map[string]int{"somegroup": 42},
			},
			chat:   &tele.Chat{Type: tele.ChatSuperGroup, Username: "somegroup"},
			wantID: 42,
		},
		{
			name: "private mode returns root for unknown group",
			cfg: runtimeConfig{
				Bot: botConfig{
					adminUsers: map[int64]struct{}{1001: {}},
				},
				groupTopics: map[string]int{"somegroup": 42},
			},
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
	t.Parallel()

	oldCfg := cfg
	cfg = defaultRuntimeConfig()
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
