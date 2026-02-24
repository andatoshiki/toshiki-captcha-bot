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
