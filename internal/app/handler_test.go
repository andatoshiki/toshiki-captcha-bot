package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tele "gopkg.in/telebot.v3"
	"toshiki-captcha-bot/internal/captcha"
	"toshiki-captcha-bot/internal/settings"
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

type mockClearContext struct {
	tele.Context
	chat      *tele.Chat
	sender    *tele.User
	message   *tele.Message
	sendErr   error
	sendCount int
	lastText  string
}

func (m *mockClearContext) Chat() *tele.Chat {
	return m.chat
}

func (m *mockClearContext) Sender() *tele.User {
	return m.sender
}

func (m *mockClearContext) Message() *tele.Message {
	return m.message
}

func (m *mockClearContext) Send(what interface{}, _ ...interface{}) error {
	m.sendCount++
	if text, ok := what.(string); ok {
		m.lastText = text
	}
	return m.sendErr
}

func mustValidatedHandlerConfig(t *testing.T, cfg settings.RuntimeConfig) settings.RuntimeConfig {
	t.Helper()
	cfg.Bot.Token = "test-token"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	return cfg
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
	if !strings.Contains(err.Error(), "load emoji asset key=does_not_exist") {
		t.Fatalf("renderCaptchaImage error = %q, want emoji-asset load error", err.Error())
	}
}

func TestOnClearAuthorizedSummary(t *testing.T) {
	oldCfg := cfg
	oldClearFn := clearManagedBotMessagesInChatFn
	oldSendFn := sendWithConfiguredTopicFn
	oldDeleteFn := deleteMessageFn
	t.Cleanup(func() {
		cfg = oldCfg
		clearManagedBotMessagesInChatFn = oldClearFn
		sendWithConfiguredTopicFn = oldSendFn
		deleteMessageFn = oldDeleteFn
	})

	cfg = mustValidatedHandlerConfig(t, func() settings.RuntimeConfig {
		c := settings.DefaultRuntimeConfig()
		c.Bot.AdminUserIDs = []int64{42}
		c.Groups = []settings.GroupTopicConfig{{ID: "@allowedgroup"}}
		return c
	}())

	managedCalled := false
	clearManagedBotMessagesInChatFn = func(chat *tele.Chat) (int, int) {
		managedCalled = true
		if chat == nil || chat.ID != -1001 {
			t.Fatalf("clearManagedBotMessagesInChatFn chat = %+v, want id -1001", chat)
		}
		return 7, 2
	}

	var summary string
	sendWithConfiguredTopicFn = func(chat *tele.Chat, what interface{}, parseMode tele.ParseMode, _ *tele.ReplyMarkup) (*tele.Message, error) {
		if chat == nil || chat.ID != -1001 {
			t.Fatalf("sendWithConfiguredTopicFn chat = %+v, want id -1001", chat)
		}
		if parseMode != tele.ModeDefault {
			t.Fatalf("parseMode = %q, want %q", parseMode, tele.ModeDefault)
		}
		text, ok := what.(string)
		if !ok {
			t.Fatalf("summary payload type = %T, want string", what)
		}
		summary = text
		return &tele.Message{Chat: chat}, nil
	}

	deleteMessageFn = func(msg tele.Editable) error {
		if msg == nil {
			t.Fatalf("deleteMessageFn got nil editable")
		}
		return nil
	}

	chat := &tele.Chat{ID: -1001, Type: tele.ChatSuperGroup, Username: "allowedgroup"}
	ctx := &mockClearContext{
		chat:    chat,
		sender:  &tele.User{ID: 42},
		message: &tele.Message{ID: 9, Chat: chat},
	}

	if err := onClear(ctx); err != nil {
		t.Fatalf("onClear returned error: %v", err)
	}

	if !managedCalled {
		t.Fatalf("clearManagedBotMessagesInChatFn was not called")
	}
	if ctx.sendCount != 0 {
		t.Fatalf("context sendCount = %d, want 0 for authorized group clear path", ctx.sendCount)
	}
	wantSummary := "Messages cleared: 7.\nWarnings: 2 cleanup operations failed."
	if summary != wantSummary {
		t.Fatalf("summary = %q, want %q", summary, wantSummary)
	}
}

func TestOnClearUnauthorizedSender(t *testing.T) {
	oldCfg := cfg
	oldClearFn := clearManagedBotMessagesInChatFn
	oldSendFn := sendWithConfiguredTopicFn
	t.Cleanup(func() {
		cfg = oldCfg
		clearManagedBotMessagesInChatFn = oldClearFn
		sendWithConfiguredTopicFn = oldSendFn
	})

	cfg = mustValidatedHandlerConfig(t, func() settings.RuntimeConfig {
		c := settings.DefaultRuntimeConfig()
		c.Bot.AdminUserIDs = []int64{42}
		c.Groups = []settings.GroupTopicConfig{{ID: "@allowedgroup"}}
		return c
	}())

	managedCalled := false
	clearManagedBotMessagesInChatFn = func(_ *tele.Chat) (int, int) {
		managedCalled = true
		return 0, 0
	}

	sendCalled := false
	sendWithConfiguredTopicFn = func(_ *tele.Chat, _ interface{}, _ tele.ParseMode, _ *tele.ReplyMarkup) (*tele.Message, error) {
		sendCalled = true
		return nil, nil
	}

	chat := &tele.Chat{ID: -1001, Type: tele.ChatSuperGroup, Username: "allowedgroup"}
	ctx := &mockClearContext{
		chat:    chat,
		sender:  &tele.User{ID: 99},
		message: &tele.Message{ID: 9, Chat: chat},
	}

	if err := onClear(ctx); err != nil {
		t.Fatalf("onClear returned error: %v", err)
	}

	if managedCalled {
		t.Fatalf("clearManagedBotMessagesInChatFn should not be called for unauthorized sender")
	}
	if sendCalled {
		t.Fatalf("sendWithConfiguredTopicFn should not be called for unauthorized sender")
	}
	if ctx.sendCount != 1 {
		t.Fatalf("context sendCount = %d, want 1", ctx.sendCount)
	}
	if !strings.Contains(ctx.lastText, "Access denied: /clear") {
		t.Fatalf("denial text = %q, expected access denied for /clear", ctx.lastText)
	}
}

func TestOnClearPrivateChatGuidance(t *testing.T) {
	oldCfg := cfg
	oldClearFn := clearManagedBotMessagesInChatFn
	oldSendFn := sendWithConfiguredTopicFn
	oldDeleteFn := deleteMessageFn
	t.Cleanup(func() {
		cfg = oldCfg
		clearManagedBotMessagesInChatFn = oldClearFn
		sendWithConfiguredTopicFn = oldSendFn
		deleteMessageFn = oldDeleteFn
	})

	cfg = mustValidatedHandlerConfig(t, func() settings.RuntimeConfig {
		c := settings.DefaultRuntimeConfig()
		c.Bot.AdminUserIDs = []int64{42}
		c.Groups = []settings.GroupTopicConfig{{ID: "@allowedgroup"}}
		return c
	}())

	managedCalled := false
	clearManagedBotMessagesInChatFn = func(_ *tele.Chat) (int, int) {
		managedCalled = true
		return 0, 0
	}

	sendCalled := false
	sendWithConfiguredTopicFn = func(_ *tele.Chat, _ interface{}, _ tele.ParseMode, _ *tele.ReplyMarkup) (*tele.Message, error) {
		sendCalled = true
		return nil, nil
	}

	deleteCalled := false
	deleteMessageFn = func(_ tele.Editable) error {
		deleteCalled = true
		return nil
	}

	chat := &tele.Chat{ID: 42, Type: tele.ChatPrivate}
	ctx := &mockClearContext{
		chat:    chat,
		sender:  &tele.User{ID: 42},
		message: &tele.Message{ID: 9, Chat: chat},
	}

	if err := onClear(ctx); err != nil {
		t.Fatalf("onClear returned error: %v", err)
	}

	if managedCalled {
		t.Fatalf("clearManagedBotMessagesInChatFn should not be called for private chat guidance path")
	}
	if sendCalled {
		t.Fatalf("sendWithConfiguredTopicFn should not be called for private chat guidance path")
	}
	if deleteCalled {
		t.Fatalf("deleteMessageFn should not be called for private chat guidance path")
	}
	if ctx.sendCount != 1 {
		t.Fatalf("context sendCount = %d, want 1 for private chat guidance", ctx.sendCount)
	}
	if ctx.lastText != "Run `/clear` inside a group chat." {
		t.Fatalf("guidance text = %q, want exact private-chat guidance", ctx.lastText)
	}
}

func TestOnClearChatNotAllowed(t *testing.T) {
	oldCfg := cfg
	oldClearFn := clearManagedBotMessagesInChatFn
	oldSendFn := sendWithConfiguredTopicFn
	oldDeleteFn := deleteMessageFn
	t.Cleanup(func() {
		cfg = oldCfg
		clearManagedBotMessagesInChatFn = oldClearFn
		sendWithConfiguredTopicFn = oldSendFn
		deleteMessageFn = oldDeleteFn
	})

	cfg = mustValidatedHandlerConfig(t, func() settings.RuntimeConfig {
		c := settings.DefaultRuntimeConfig()
		c.Bot.AdminUserIDs = []int64{42}
		c.Groups = []settings.GroupTopicConfig{{ID: "@allowedgroup"}}
		return c
	}())

	managedCalled := false
	clearManagedBotMessagesInChatFn = func(_ *tele.Chat) (int, int) {
		managedCalled = true
		return 0, 0
	}

	sendCalled := false
	sendWithConfiguredTopicFn = func(_ *tele.Chat, _ interface{}, _ tele.ParseMode, _ *tele.ReplyMarkup) (*tele.Message, error) {
		sendCalled = true
		return nil, nil
	}

	deleteCalled := false
	deleteMessageFn = func(_ tele.Editable) error {
		deleteCalled = true
		return nil
	}

	chat := &tele.Chat{ID: -1002, Type: tele.ChatSuperGroup, Username: "othergroup"}
	ctx := &mockClearContext{
		chat:    chat,
		sender:  &tele.User{ID: 42},
		message: &tele.Message{ID: 10, Chat: chat},
	}

	if err := onClear(ctx); err != nil {
		t.Fatalf("onClear returned error: %v", err)
	}

	if managedCalled {
		t.Fatalf("clearManagedBotMessagesInChatFn should not be called for not-allowed chat")
	}
	if sendCalled {
		t.Fatalf("sendWithConfiguredTopicFn should not be called for not-allowed chat")
	}
	if deleteCalled {
		t.Fatalf("deleteMessageFn should not be called for not-allowed chat")
	}
	if ctx.sendCount != 0 {
		t.Fatalf("context sendCount = %d, want 0 for not-allowed chat path", ctx.sendCount)
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

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func TestIsTimeoutLikeError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil",
			err:  nil,
			want: false,
		},
		{
			name: "context deadline exceeded",
			err:  context.DeadlineExceeded,
			want: true,
		},
		{
			name: "net timeout",
			err:  timeoutErr{},
			want: true,
		},
		{
			name: "telegram client timeout string",
			err:  fmt.Errorf("telebot: Post ...: context deadline exceeded (Client.Timeout exceeded while awaiting headers)"),
			want: true,
		},
		{
			name: "non-timeout",
			err:  errors.New("bad request"),
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := isTimeoutLikeError(tt.err)
			if got != tt.want {
				t.Fatalf("isTimeoutLikeError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestApplyCaptchaRestriction(t *testing.T) {
	t.Parallel()

	member := &tele.ChatMember{
		User:   &tele.User{ID: 42},
		Rights: tele.NoRestrictions(),
	}

	start := time.Now().Unix()
	applyCaptchaRestriction(member, 2*time.Minute)

	if member.RestrictedUntil <= start {
		t.Fatalf("restricted_until = %d, expected > %d", member.RestrictedUntil, start)
	}
	if member.Rights != tele.NoRights() {
		t.Fatalf("rights = %+v, want %+v", member.Rights, tele.NoRights())
	}
}

func TestBindCaptchaMessageIfUnset(t *testing.T) {
	t.Parallel()

	t.Run("binds when message id is unknown", func(t *testing.T) {
		t.Parallel()

		status := &captcha.JoinStatus{}
		msg := &tele.Message{
			ID:   55,
			Chat: &tele.Chat{ID: -1001},
		}

		changed := bindCaptchaMessageIfUnset(status, msg)
		if !changed {
			t.Fatalf("expected bind to report changed=true")
		}
		if status.CaptchaMessage.ID != 55 {
			t.Fatalf("captcha message id = %d, want 55", status.CaptchaMessage.ID)
		}
	})

	t.Run("does not overwrite when already bound", func(t *testing.T) {
		t.Parallel()

		status := &captcha.JoinStatus{
			CaptchaMessage: tele.Message{ID: 10},
		}
		msg := &tele.Message{ID: 99}

		changed := bindCaptchaMessageIfUnset(status, msg)
		if changed {
			t.Fatalf("expected bind to report changed=false")
		}
		if status.CaptchaMessage.ID != 10 {
			t.Fatalf("captcha message id = %d, want 10", status.CaptchaMessage.ID)
		}
	})
}

func TestNewJoinStatus(t *testing.T) {
	t.Parallel()

	user := &tele.User{
		ID:        42,
		FirstName: "Alice",
		LastName:  "Bob",
	}
	chat := &tele.Chat{ID: -100123}
	challenge := captchaChallenge{
		AnswerKeys: []string{"u1", "u2", "u3", "u4"},
		Buttons: []tele.InlineButton{
			{Unique: "u1"},
			{Unique: "u2"},
		},
	}
	message := tele.Message{
		ID:   77,
		Chat: chat,
	}

	status := newJoinStatus(user, chat, challenge, message, true)
	if status.UserID != 42 {
		t.Fatalf("user id = %d, want 42", status.UserID)
	}
	if status.ChatID != -100123 {
		t.Fatalf("chat id = %d, want -100123", status.ChatID)
	}
	if !status.ManualChallenge {
		t.Fatalf("manual challenge = %t, want true", status.ManualChallenge)
	}
	if status.CaptchaMessage.ID != 77 {
		t.Fatalf("captcha message id = %d, want 77", status.CaptchaMessage.ID)
	}
	if status.UserFullName != "Alice Bob" {
		t.Fatalf("user full name = %q, want %q", status.UserFullName, "Alice Bob")
	}
	if len(status.CaptchaAnswer) != 4 {
		t.Fatalf("captcha answer count = %d, want 4", len(status.CaptchaAnswer))
	}
}

func TestShouldBanOnCaptchaFailure(t *testing.T) {
	t.Parallel()

	if got := shouldBanOnCaptchaFailure(captcha.JoinStatus{}); !got {
		t.Fatalf("shouldBanOnCaptchaFailure(non-manual) = %t, want true", got)
	}
	if got := shouldBanOnCaptchaFailure(captcha.JoinStatus{ManualChallenge: true}); got {
		t.Fatalf("shouldBanOnCaptchaFailure(manual) = %t, want false", got)
	}
}

func TestCaptchaFailureNoticeText(t *testing.T) {
	t.Parallel()

	status := captcha.JoinStatus{
		UserID:       42,
		UserFullName: "Alice",
	}

	bannedText := captchaFailureNoticeText(status, true, 15*time.Second)
	if !strings.Contains(bannedText, "has been banned") {
		t.Fatalf("banned notice missing ban statement: %q", bannedText)
	}
	if !strings.Contains(bannedText, "15 seconds") {
		t.Fatalf("banned notice missing ttl text: %q", bannedText)
	}

	manualText := captchaFailureNoticeText(status, false, 15*time.Second)
	if !strings.Contains(manualText, "[Alice](tg://user?id=42) captcha failed.") {
		t.Fatalf("manual notice missing failure statement: %q", manualText)
	}
	if strings.Contains(manualText, "has been banned") {
		t.Fatalf("manual notice should not include ban statement: %q", manualText)
	}
}

func TestCaptchaSuccessCallbackText(t *testing.T) {
	t.Parallel()

	normalText := captchaSuccessCallbackText(captcha.JoinStatus{})
	if normalText != "Successfully joined." {
		t.Fatalf("normal success text = %q, want %q", normalText, "Successfully joined.")
	}

	manualText := captchaSuccessCallbackText(captcha.JoinStatus{ManualChallenge: true})
	if manualText != "Manual test captcha completed successfully." {
		t.Fatalf("manual success text = %q, want %q", manualText, "Manual test captcha completed successfully.")
	}
}

func TestNotYourCaptchaCallbackResponse(t *testing.T) {
	t.Parallel()

	resp := notYourCaptchaCallbackResponse()
	if resp == nil {
		t.Fatalf("notYourCaptchaCallbackResponse returned nil")
	}
	if !resp.ShowAlert {
		t.Fatalf("show alert = %t, want true", resp.ShowAlert)
	}
	if !strings.Contains(resp.Text, "not your captcha") {
		t.Fatalf("response text = %q, expected not-your-captcha message", resp.Text)
	}
}

func TestCaptchaTimeoutNoticeText(t *testing.T) {
	t.Parallel()

	status := captcha.JoinStatus{
		UserID:       42,
		UserFullName: "Alice",
	}

	timeoutText := captchaTimeoutNoticeText(status)
	if !strings.Contains(timeoutText, "[Alice](tg://user?id=42)") {
		t.Fatalf("timeout notice missing mention: %q", timeoutText)
	}
	if !strings.Contains(timeoutText, "did not resolve the challenge in time") {
		t.Fatalf("timeout notice missing timeout reason: %q", timeoutText)
	}
}

func TestCaptchaSendTimeoutMarker(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("%w: %v", errCaptchaSendTimeout, context.DeadlineExceeded)
	if !errors.Is(wrapped, errCaptchaSendTimeout) {
		t.Fatalf("expected wrapped error to match errCaptchaSendTimeout")
	}
}

func TestResolveTestCaptchaTargetFromReply(t *testing.T) {
	t.Parallel()

	target := &tele.User{ID: 2}

	t.Run("resolves replied target without username", func(t *testing.T) {
		t.Parallel()

		msg := &tele.Message{
			ReplyTo: &tele.Message{
				Sender: target,
			},
		}
		got, err := resolveTestCaptchaTargetFromReply(msg)
		if err != nil {
			t.Fatalf("resolveTestCaptchaTargetFromReply returned error: %v", err)
		}
		if got == nil || got.ID != target.ID {
			t.Fatalf("resolved user id = %v, want %d", got, target.ID)
		}
	})

	t.Run("fails when no reply target", func(t *testing.T) {
		t.Parallel()

		msg := &tele.Message{}
		_, err := resolveTestCaptchaTargetFromReply(msg)
		if err == nil {
			t.Fatalf("expected error for missing reply target")
		}
	})

	t.Run("fails when command message is nil", func(t *testing.T) {
		t.Parallel()

		_, err := resolveTestCaptchaTargetFromReply(nil)
		if err == nil {
			t.Fatalf("expected error for nil command message")
		}
	})
}

func TestIsBotCaptchaTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		user *tele.User
		want bool
	}{
		{
			name: "nil user",
			user: nil,
			want: false,
		},
		{
			name: "native bot flag",
			user: &tele.User{ID: 1, IsBot: true},
			want: true,
		},
		{
			name: "username suffix bot",
			user: &tele.User{ID: 2, Username: "helperbot"},
			want: true,
		},
		{
			name: "username suffix bot uppercase",
			user: &tele.User{ID: 3, Username: "HelperBOT"},
			want: true,
		},
		{
			name: "regular user",
			user: &tele.User{ID: 4, Username: "andatoshiki"},
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isBotCaptchaTarget(tt.user)
			if got != tt.want {
				t.Fatalf("isBotCaptchaTarget(%+v) = %t, want %t", tt.user, got, tt.want)
			}
		})
	}
}
