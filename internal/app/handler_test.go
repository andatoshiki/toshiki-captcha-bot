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

func TestBuildCaptchaChallenge(t *testing.T) {
	t.Parallel()

	challenge, err := buildCaptchaChallenge(4, 6)
	if err != nil {
		t.Fatalf("buildCaptchaChallenge returned error: %v", err)
	}

	if len(challenge.AnswerKeys) != 4 {
		t.Fatalf("answer key count = %d, want 4", len(challenge.AnswerKeys))
	}
	if len(challenge.Buttons) != 10 {
		t.Fatalf("button count = %d, want 10", len(challenge.Buttons))
	}
	if challenge.Markup == nil {
		t.Fatalf("markup is nil")
	}
	if len(challenge.Markup.InlineKeyboard) == 0 {
		t.Fatalf("markup has no inline keyboard rows")
	}
	if len(challenge.ImageBytes) == 0 {
		t.Fatalf("image bytes are empty")
	}

	buttonsByKey := make(map[string]struct{}, len(challenge.Buttons))
	for _, button := range challenge.Buttons {
		buttonsByKey[button.Unique] = struct{}{}
	}
	for _, answerKey := range challenge.AnswerKeys {
		if _, ok := buttonsByKey[answerKey]; !ok {
			t.Fatalf("answer key %q is not present in challenge buttons", answerKey)
		}
	}
}

func TestApplyCaptchaChallenge(t *testing.T) {
	t.Parallel()

	status := captcha.JoinStatus{
		SolvedCaptcha: 3,
		FailCaptcha:   2,
	}
	challenge := captchaChallenge{
		AnswerKeys: []string{"u1", "u2", "u3", "u4"},
		Buttons: []tele.InlineButton{
			{Text: "A", Unique: "u1"},
			{Text: "B", Unique: "u2"},
		},
	}
	message := tele.Message{ID: 77}

	applyCaptchaChallenge(&status, challenge, message)

	if status.SolvedCaptcha != 0 {
		t.Fatalf("solved captcha = %d, want 0", status.SolvedCaptcha)
	}
	if status.FailCaptcha != 2 {
		t.Fatalf("fail captcha = %d, want 2", status.FailCaptcha)
	}
	if status.CaptchaMessage.ID != 77 {
		t.Fatalf("captcha message id = %d, want 77", status.CaptchaMessage.ID)
	}
	if len(status.CaptchaAnswer) != 4 {
		t.Fatalf("captcha answer count = %d, want 4", len(status.CaptchaAnswer))
	}
	if len(status.Buttons) != 2 {
		t.Fatalf("button count = %d, want 2", len(status.Buttons))
	}

	challenge.Buttons[0].Text = "mutated"
	if status.Buttons[0].Text == "mutated" {
		t.Fatalf("status buttons should be copied, got shared underlying data")
	}
}

func TestCaptchaMarkupFromButtons(t *testing.T) {
	t.Parallel()

	t.Run("single row when up to five buttons", func(t *testing.T) {
		t.Parallel()

		buttons := []tele.InlineButton{
			{Unique: "u1"},
			{Unique: "u2"},
			{Unique: "u3"},
			{Unique: "u4"},
			{Unique: "u5"},
		}
		markup := captchaMarkupFromButtons(buttons)

		if len(markup.InlineKeyboard) != 1 {
			t.Fatalf("rows = %d, want 1", len(markup.InlineKeyboard))
		}
		if len(markup.InlineKeyboard[0]) != 5 {
			t.Fatalf("row size = %d, want 5", len(markup.InlineKeyboard[0]))
		}
	})

	t.Run("two rows when more than five buttons", func(t *testing.T) {
		t.Parallel()

		buttons := []tele.InlineButton{
			{Unique: "u1"},
			{Unique: "u2"},
			{Unique: "u3"},
			{Unique: "u4"},
			{Unique: "u5"},
			{Unique: "u6"},
			{Unique: "u7"},
		}
		markup := captchaMarkupFromButtons(buttons)

		if len(markup.InlineKeyboard) != 2 {
			t.Fatalf("rows = %d, want 2", len(markup.InlineKeyboard))
		}
		if len(markup.InlineKeyboard[0]) != 5 {
			t.Fatalf("row0 size = %d, want 5", len(markup.InlineKeyboard[0]))
		}
		if len(markup.InlineKeyboard[1]) != 2 {
			t.Fatalf("row1 size = %d, want 2", len(markup.InlineKeyboard[1]))
		}
	})
}
