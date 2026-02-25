package app

import (
	"fmt"
	"testing"
	"time"

	"github.com/codenoid/minikv"
	tele "gopkg.in/telebot.v3"
	"toshiki-captcha-bot/internal/captcha"
)

func TestCleanupPendingCaptchaForUserDeletesState(t *testing.T) {
	t.Parallel()

	origDB := db
	origBot := bot
	t.Cleanup(func() {
		db = origDB
		bot = origBot
	})

	db = minikv.New(time.Minute, time.Hour)
	bot = nil

	chat := &tele.Chat{ID: -100123}
	user := &tele.User{ID: 1001}
	key := fmt.Sprintf("%v-%v", user.ID, chat.ID)
	db.Set(key, captcha.JoinStatus{
		UserID:  user.ID,
		ChatID:  chat.ID,
		Buttons: []tele.InlineButton{{Unique: "u1"}},
		CaptchaMessage: tele.Message{
			ID:   88,
			Chat: chat,
		},
	}, time.Minute)

	if _, found := db.Get(key); !found {
		t.Fatalf("expected captcha state to exist before cleanup")
	}

	cleanupPendingCaptchaForUser(chat, user)

	if _, found := db.Get(key); found {
		t.Fatalf("expected captcha state to be deleted after cleanup")
	}
}

func TestCleanupPendingCaptchaForUserNoopForMissingEntry(t *testing.T) {
	t.Parallel()

	origDB := db
	origBot := bot
	t.Cleanup(func() {
		db = origDB
		bot = origBot
	})

	db = minikv.New(time.Minute, time.Hour)
	bot = nil

	chat := &tele.Chat{ID: -100123}
	user := &tele.User{ID: 1001}
	cleanupPendingCaptchaForUser(chat, user)
}
