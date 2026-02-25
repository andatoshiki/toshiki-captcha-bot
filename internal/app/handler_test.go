package app

import (
	"errors"
	"strings"
	"testing"

	tele "gopkg.in/telebot.v3"
	"toshiki-captcha-bot/internal/captcha"
)

func TestIsNextCaptchaAnswer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		status       captcha.JoinStatus
		answer       string
		wantOK       bool
		wantExpected string
	}{
		{
			name: "first step correct",
			status: captcha.JoinStatus{
				CaptchaAnswer: []string{"u1", "u2", "u3", "u4"},
				SolvedCaptcha: 0,
			},
			answer:       "u1",
			wantOK:       true,
			wantExpected: "u1",
		},
		{
			name: "first step wrong",
			status: captcha.JoinStatus{
				CaptchaAnswer: []string{"u1", "u2", "u3", "u4"},
				SolvedCaptcha: 0,
			},
			answer:       "u2",
			wantOK:       false,
			wantExpected: "u1",
		},
		{
			name: "duplicate previous tap is wrong",
			status: captcha.JoinStatus{
				CaptchaAnswer: []string{"u1", "u2", "u3", "u4"},
				SolvedCaptcha: 1,
			},
			answer:       "u1",
			wantOK:       false,
			wantExpected: "u2",
		},
		{
			name: "next step correct",
			status: captcha.JoinStatus{
				CaptchaAnswer: []string{"u1", "u2", "u3", "u4"},
				SolvedCaptcha: 2,
			},
			answer:       "u3",
			wantOK:       true,
			wantExpected: "u3",
		},
		{
			name: "out of range solved index",
			status: captcha.JoinStatus{
				CaptchaAnswer: []string{"u1", "u2", "u3", "u4"},
				SolvedCaptcha: 4,
			},
			answer:       "u4",
			wantOK:       false,
			wantExpected: "",
		},
		{
			name: "empty answers",
			status: captcha.JoinStatus{
				CaptchaAnswer: []string{},
				SolvedCaptcha: 0,
			},
			answer:       "u1",
			wantOK:       false,
			wantExpected: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotOK, gotExpected := isNextCaptchaAnswer(tt.status, tt.answer)
			if gotOK != tt.wantOK {
				t.Fatalf("ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotExpected != tt.wantExpected {
				t.Fatalf("expected = %q, want %q", gotExpected, tt.wantExpected)
			}
		})
	}
}

type mockAdminCommandResponder struct {
	chat      *tele.Chat
	sender    *tele.User
	sendErr   error
	sendCount int
	lastText  string
}

func (m *mockAdminCommandResponder) Chat() *tele.Chat {
	return m.chat
}

func (m *mockAdminCommandResponder) Sender() *tele.User {
	return m.sender
}

func (m *mockAdminCommandResponder) Send(what interface{}, _ ...interface{}) error {
	m.sendCount++
	if text, ok := what.(string); ok {
		m.lastText = text
	}
	return m.sendErr
}

func TestRespondAdminOnlyCommandDenied(t *testing.T) {
	t.Parallel()

	t.Run("sends denial message for valid context", func(t *testing.T) {
		t.Parallel()

		ctx := &mockAdminCommandResponder{
			chat:   &tele.Chat{ID: -1001},
			sender: &tele.User{ID: 42},
		}

		respondAdminOnlyCommandDenied(ctx, "/ping")

		if ctx.sendCount != 1 {
			t.Fatalf("sendCount = %d, want 1", ctx.sendCount)
		}
		if ctx.lastText != adminOnlyCommandErrorText("/ping") {
			t.Fatalf("lastText = %q, want %q", ctx.lastText, adminOnlyCommandErrorText("/ping"))
		}
	})

	t.Run("skips when sender missing", func(t *testing.T) {
		t.Parallel()

		ctx := &mockAdminCommandResponder{
			chat: &tele.Chat{ID: -1001},
		}

		respondAdminOnlyCommandDenied(ctx, "/testcaptcha")

		if ctx.sendCount != 0 {
			t.Fatalf("sendCount = %d, want 0", ctx.sendCount)
		}
	})

	t.Run("safe when send returns error", func(t *testing.T) {
		t.Parallel()

		ctx := &mockAdminCommandResponder{
			chat:    &tele.Chat{ID: -1001},
			sender:  &tele.User{ID: 42},
			sendErr: errors.New("send failed"),
		}

		respondAdminOnlyCommandDenied(ctx, "/ping")

		if ctx.sendCount != 1 {
			t.Fatalf("sendCount = %d, want 1", ctx.sendCount)
		}
	})
}

func TestRenderCaptchaImageWithMissingAsset(t *testing.T) {
	t.Parallel()

	_, err := renderCaptchaImage([]string{"does_not_exist"})
	if err == nil {
		t.Fatalf("renderCaptchaImage expected error for missing asset key")
	}
	if !strings.Contains(err.Error(), "merge captcha layers") {
		t.Fatalf("renderCaptchaImage error = %q, want merge error", err.Error())
	}
}
