package main

import (
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

	const configuredThreadID = 42

	tests := []struct {
		name    string
		chat    *tele.Chat
		wantID  int
	}{
		{
			name:   "nil chat falls back to root",
			chat:   nil,
			wantID: 0,
		},
		{
			name:   "private chat ignores configured topic",
			chat:   &tele.Chat{Type: tele.ChatPrivate},
			wantID: 0,
		},
		{
			name:   "group chat uses configured topic",
			chat:   &tele.Chat{Type: tele.ChatGroup},
			wantID: configuredThreadID,
		},
		{
			name:   "supergroup chat uses configured topic",
			chat:   &tele.Chat{Type: tele.ChatSuperGroup},
			wantID: configuredThreadID,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := topicThreadIDForChat(tt.chat, configuredThreadID); got != tt.wantID {
				t.Fatalf("topicThreadIDForChat() = %d, want %d", got, tt.wantID)
			}
		})
	}
}
