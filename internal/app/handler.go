package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/jpeg"
	"log"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	gim "github.com/codenoid/goimagemerge"
	tele "gopkg.in/telebot.v3"
	"toshiki-captcha-bot/internal/captcha"
)

type adminCommandResponder interface {
	Chat() *tele.Chat
	Sender() *tele.User
	Send(what interface{}, opts ...interface{}) error
}

const (
	captchaAnswerCount = 4
	captchaDecoyCount  = 6
)

type captchaChallenge struct {
	AnswerKeys []string
	Buttons    []tele.InlineButton
	Markup     *tele.ReplyMarkup
	ImageBytes []byte
}

func onPing(c tele.Context) error {
	if c == nil || c.Chat() == nil {
		log.Printf("warn: ping skipped reason=missing_chat_context")
		return nil
	}
	if c.Sender() == nil {
		log.Printf("warn: ping skipped reason=missing_sender chat_id=%d", c.Chat().ID)
		return nil
	}
	if leaveIfUnsupportedPrivateGroup(c.Chat(), "ping") {
		return nil
	}
	if !isAllowedCommandChat(c.Chat()) {
		logAccessDenied(c, "ping_chat_not_allowed")
		if isGroupChat(c.Chat()) {
			leaveChat(c.Chat(), "unauthorized_group")
		}
		return nil
	}

	if !isSenderAllowed(c) {
		logAccessDenied(c, "ping_sender_not_allowed")
		respondAdminOnlyCommandDenied(c, "/ping")
		return nil
	}

	chatID := c.Chat().ID
	chatType := c.Chat().Type
	userID := int64(0)
	username := ""
	messageID := 0
	if c.Sender() != nil {
		userID = c.Sender().ID
		username = c.Sender().Username
	}
	if c.Message() != nil {
		messageID = c.Message().ID
	}
	threadID := 0
	if c.Chat().Type != tele.ChatPrivate {
		if incoming := c.Message(); incoming != nil {
			threadID = incoming.ThreadID
		}
	}
	log.Printf(
		"Ping received chat_id=%d chat_type=%s user_id=%d username=%q message_id=%d request_thread_id=%d configured_topic_thread_id=%d",
		chatID,
		chatType,
		userID,
		username,
		messageID,
		threadID,
		topicThreadIDForChat(c.Chat()),
	)

	start := time.Now()
	opts := buildSendOptionsWithTopic(tele.ModeDefault, nil, threadID)
	log.Printf("Ping send attempt chat_id=%d user_id=%d thread_id=%d", chatID, userID, threadID)
	msg, err := bot.Send(c.Chat(), "pong...", opts)
	if err != nil && threadID != 0 && strings.Contains(err.Error(), "message thread not found") {
		log.Printf(
			"warn: ping send failed with thread not found chat_id=%d user_id=%d thread_id=%d err=%v fallback=chat_root",
			chatID,
			userID,
			threadID,
			err,
		)
		// Fallback to chat root when thread reference is stale or invalid.
		msg, err = bot.Send(c.Chat(), "pong...", buildSendOptionsWithTopic(tele.ModeDefault, nil, 0))
	}
	if err != nil {
		log.Printf("warn: failed to send ping response chat_id=%d err=%v", chatID, err)
		return nil
	}

	latencyMS := time.Since(start).Milliseconds()
	log.Printf("Ping sent chat_id=%d user_id=%d response_message_id=%d latency_ms=%d", chatID, userID, msg.ID, latencyMS)
	if _, err := bot.Edit(msg, fmt.Sprintf("pong %d ms", latencyMS)); err != nil {
		log.Printf("warn: failed to edit ping response chat_id=%d message_id=%d err=%v", chatID, msg.ID, err)
		return nil
	}
	log.Printf("Ping completed chat_id=%d user_id=%d response_message_id=%d latency_ms=%d", chatID, userID, msg.ID, latencyMS)
	return nil
}

func onTestCaptcha(c tele.Context) error {
	if c == nil || c.Chat() == nil {
		log.Printf("warn: testcaptcha skipped reason=missing_chat_context")
		return nil
	}
	if c.Sender() == nil {
		log.Printf("warn: testcaptcha skipped reason=missing_sender chat_id=%d", c.Chat().ID)
		return nil
	}
	if leaveIfUnsupportedPrivateGroup(c.Chat(), "testcaptcha") {
		return nil
	}
	if !isAllowedCommandChat(c.Chat()) {
		logAccessDenied(c, "testcaptcha_chat_not_allowed")
		if isGroupChat(c.Chat()) {
			leaveChat(c.Chat(), "unauthorized_group")
		}
		return nil
	}
	if !isSenderAllowed(c) {
		logAccessDenied(c, "testcaptcha_sender_not_allowed")
		respondAdminOnlyCommandDenied(c, "/testcaptcha")
		return nil
	}
	if c.Chat().Type == tele.ChatPrivate {
		log.Printf("warn: testcaptcha skipped reason=private_chat_requires_group chat_id=%d user_id=%d", c.Chat().ID, c.Sender().ID)
		return nil
	}
	log.Printf("Manual captcha trigger chat_id=%d user_id=%d", c.Chat().ID, c.Sender().ID)
	return onJoin(c)
}

func respondAdminOnlyCommandDenied(c adminCommandResponder, command string) {
	if c == nil || c.Chat() == nil || c.Sender() == nil {
		return
	}
	if err := c.Send(adminOnlyCommandErrorText(command)); err != nil {
		log.Printf(
			"warn: failed to send unauthorized response command=%s chat_id=%d user_id=%d err=%v",
			command,
			c.Chat().ID,
			c.Sender().ID,
			err,
		)
	}
}

func onJoin(c tele.Context) error {
	if c == nil || c.Chat() == nil {
		return nil
	}
	if c.Sender() == nil {
		log.Printf("warn: join skipped reason=missing_sender chat_id=%d", c.Chat().ID)
		return nil
	}
	if c.Message() == nil {
		log.Printf("warn: join skipped reason=missing_message chat_id=%d user_id=%d", c.Chat().ID, c.Sender().ID)
		return nil
	}

	if c.Chat().Type == tele.ChatPrivate {
		return nil
	}

	if leaveIfUnsupportedPrivateGroup(c.Chat(), "join") {
		return nil
	}

	if !isContextAuthorized(c) {
		logAccessDenied(c, "join")
		leaveChat(c.Chat(), "unauthorized_group")
		return nil
	}

	// delete any incoming message before challenge solved
	if err := bot.Delete(c.Message()); err != nil {
		log.Printf("warn: failed to delete incoming join message chat_id=%d user_id=%d err=%v", c.Chat().ID, c.Sender().ID, err)
	}

	// kvID is combination of user id and chat id
	kvID := fmt.Sprintf("%v-%v", c.Sender().ID, c.Chat().ID)

	// skip captcha-generation if data still exist
	if _, found := db.Get(kvID); found {
		c.Respond(&tele.CallbackResponse{Text: "Please solve existing captcha.", ShowAlert: true})
		log.Printf("Captcha already pending chat_id=%d user_id=%d", c.Chat().ID, c.Sender().ID)
		return nil
	}

	chatMember, err := bot.ChatMemberOf(c.Chat(), c.Sender())
	if err != nil {
		log.Printf("warn: failed to load member state for restriction chat_id=%d user_id=%d err=%v", c.Chat().ID, c.Sender().ID, err)
		return nil
	}
	originalMember := *chatMember
	applyCaptchaRestriction(chatMember, cfg.Captcha.Expiration)
	if err := bot.Restrict(c.Chat(), chatMember); err != nil {
		log.Printf("warn: failed to restrict user chat_id=%d user_id=%d err=%v", c.Chat().ID, c.Sender().ID, err)
		return nil
	}
	log.Printf("User restricted pending captcha chat_id=%d user_id=%d until=%d", c.Chat().ID, c.Sender().ID, chatMember.RestrictedUntil)

	challenge, err := buildCaptchaChallenge(captchaAnswerCount, captchaDecoyCount)
	if err != nil {
		log.Printf("error: captcha generation failed chat_id=%d user_id=%d err=%v", c.Chat().ID, c.Sender().ID, err)
		restoreUserRestriction(c.Chat(), c.Sender(), &originalMember, "captcha_generation_failed")
		return nil
	}

	msg, err := sendCaptchaChallengeWithRetry(c.Chat(), c.Sender(), challenge.ImageBytes, genCaption(c.Sender()), challenge.Markup)
	if err != nil {
		log.Printf("error: failed to send captcha challenge chat_id=%d user_id=%d err=%v", c.Chat().ID, c.Sender().ID, err)
		restoreUserRestriction(c.Chat(), c.Sender(), &originalMember, "captcha_send_failed")
		return nil
	}

	status := captcha.JoinStatus{
		UserID: c.Sender().ID,
		ChatID: c.Chat().ID,
	}
	applyCaptchaChallenge(&status, challenge, *msg)

	status.UserFullName = c.Sender().FirstName + " " + c.Sender().LastName
	status.UserFullName = sanitizeName(status.UserFullName)

	db.Set(kvID, status, cfg.Captcha.Expiration)
	log.Printf(
		"Captcha issued chat_id=%d user_id=%d challenge_message_id=%d answer_count=%d topic_thread_id=%d",
		c.Chat().ID,
		c.Sender().ID,
		msg.ID,
		len(status.CaptchaAnswer),
		topicThreadIDForChat(c.Chat()),
	)

	return nil
}

func applyCaptchaRestriction(member *tele.ChatMember, duration time.Duration) {
	if member == nil {
		return
	}
	member.Rights = tele.NoRights()
	member.RestrictedUntil = time.Now().Add(duration).Unix()
}

func restoreUserRestriction(chat *tele.Chat, user *tele.User, member *tele.ChatMember, reason string) {
	if chat == nil || user == nil || member == nil {
		return
	}
	if bot == nil {
		log.Printf(
			"warn: restore skipped reason=bot_not_initialized chat_id=%d user_id=%d restore_reason=%s",
			chat.ID,
			user.ID,
			reason,
		)
		return
	}
	if member.User == nil {
		member.User = user
	}
	if err := bot.Restrict(chat, member); err != nil {
		log.Printf(
			"warn: failed to restore user restriction state chat_id=%d user_id=%d reason=%s err=%v",
			chat.ID,
			user.ID,
			reason,
			err,
		)
		return
	}
	log.Printf(
		"User restriction state restored chat_id=%d user_id=%d reason=%s",
		chat.ID,
		user.ID,
		reason,
	)
}

func sendCaptchaChallengeWithRetry(chat *tele.Chat, user *tele.User, imageBytes []byte, caption string, markup *tele.ReplyMarkup) (*tele.Message, error) {
	const maxAttempts = 3
	var lastErr error

	chatID := int64(0)
	userID := int64(0)
	if chat != nil {
		chatID = chat.ID
	}
	if user != nil {
		userID = user.ID
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		file := tele.FromReader(bytes.NewReader(imageBytes))
		photo := &tele.Photo{File: file}
		photo.Caption = caption

		msg, err := sendWithConfiguredTopic(chat, photo, tele.ModeMarkdown, markup)
		if err == nil {
			if attempt > 1 {
				log.Printf("Captcha challenge send recovered chat_id=%d user_id=%d attempt=%d", chatID, userID, attempt)
			}
			return msg, nil
		}

		lastErr = err
		if !isTimeoutLikeError(err) || attempt == maxAttempts {
			break
		}

		backoff := time.Duration(attempt) * time.Second
		log.Printf(
			"warn: captcha challenge send timeout chat_id=%d user_id=%d attempt=%d next_retry_in=%s err=%v",
			chatID,
			userID,
			attempt,
			backoff,
			err,
		)
		time.Sleep(backoff)
	}

	return nil, lastErr
}

func isTimeoutLikeError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "context deadline exceeded") ||
		strings.Contains(lower, "client.timeout exceeded while awaiting headers")
}

func buildCaptchaChallenge(answerCount, decoyCount int) (captchaChallenge, error) {
	challengeCount := answerCount + decoyCount
	if challengeCount <= 0 || answerCount <= 0 {
		return captchaChallenge{}, fmt.Errorf("invalid captcha challenge size answer_count=%d decoy_count=%d", answerCount, decoyCount)
	}

	emojiKeys := make([]string, 0, len(captcha.Emojis))
	for key := range captcha.Emojis {
		emojiKeys = append(emojiKeys, key)
	}
	if len(emojiKeys) < challengeCount {
		return captchaChallenge{}, fmt.Errorf(
			"insufficient emoji pool available=%d required=%d",
			len(emojiKeys),
			challengeCount,
		)
	}

	rand.Shuffle(len(emojiKeys), func(i, j int) {
		emojiKeys[i], emojiKeys[j] = emojiKeys[j], emojiKeys[i]
	})

	answerKeys := append([]string(nil), emojiKeys[:answerCount]...)
	challengeKeys := append([]string(nil), emojiKeys[:challengeCount]...)
	rand.Shuffle(len(challengeKeys), func(i, j int) {
		challengeKeys[i], challengeKeys[j] = challengeKeys[j], challengeKeys[i]
	})

	imgBytes, err := renderCaptchaImage(answerKeys)
	if err != nil {
		return captchaChallenge{}, fmt.Errorf("render captcha image: %w", err)
	}

	buttons := make([]tele.InlineButton, 0, len(challengeKeys))
	for _, key := range challengeKeys {
		emoji := captcha.Emojis[key]
		buttons = append(buttons, tele.InlineButton{Text: emoji, Unique: key})
	}

	return captchaChallenge{
		AnswerKeys: answerKeys,
		Buttons:    buttons,
		Markup:     captchaMarkupFromButtons(buttons),
		ImageBytes: imgBytes,
	}, nil
}

func captchaMarkupFromButtons(buttons []tele.InlineButton) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{
		Selective:      true,
		InlineKeyboard: [][]tele.InlineButton{},
	}
	if len(buttons) == 0 {
		return markup
	}
	if len(buttons) <= 5 {
		markup.InlineKeyboard = append(markup.InlineKeyboard, append([]tele.InlineButton(nil), buttons...))
		return markup
	}
	markup.InlineKeyboard = append(
		markup.InlineKeyboard,
		append([]tele.InlineButton(nil), buttons[:5]...),
		append([]tele.InlineButton(nil), buttons[5:]...),
	)
	return markup
}

func applyCaptchaChallenge(status *captcha.JoinStatus, challenge captchaChallenge, message tele.Message) {
	if status == nil {
		return
	}
	status.CaptchaAnswer = make([]string, 0, len(challenge.AnswerKeys))
	for _, key := range challenge.AnswerKeys {
		status.CaptchaAnswer = append(status.CaptchaAnswer, strings.TrimSpace(key))
	}
	status.SolvedCaptcha = 0
	status.CaptchaMessage = message
	status.Buttons = append([]tele.InlineButton(nil), challenge.Buttons...)
}

func renderCaptchaImage(answerKeys []string) ([]byte, error) {
	captchaGrids := make([]*gim.Grid, 0, len(answerKeys))
	for i, key := range answerKeys {
		x := 10
		if i > 0 {
			x = i * 100
		}
		captchaGrids = append(captchaGrids, &gim.Grid{
			ImageFilePath: resolveAssetPath(fmt.Sprintf("./assets/image/emoji/%v.png", key)),
			OffsetX:       x,
			OffsetY:       120,
			Rotate:        float64(rand.Intn(200)),
		})
	}

	grids := []*gim.Grid{
		{
			ImageFilePath: resolveAssetPath("./gopherbg.jpg"),
			Grids:         captchaGrids,
		},
	}

	rgba, err := gim.New(grids, 1, 1).Merge()
	if err != nil {
		return nil, fmt.Errorf("merge captcha layers: %w", err)
	}

	var img bytes.Buffer
	if err := jpeg.Encode(&img, rgba, &jpeg.Options{Quality: 100}); err != nil {
		return nil, fmt.Errorf("encode captcha image: %w", err)
	}

	return img.Bytes(), nil
}

func resolveAssetPath(relPath string) string {
	clean := filepath.Clean(relPath)
	clean = strings.TrimPrefix(clean, "./")

	candidates := []string{
		filepath.Clean(relPath),
		filepath.Join(".", clean),
		filepath.Join("..", clean),
		filepath.Join("..", "..", clean),
		filepath.Join("..", "..", "..", clean),
	}

	if exePath, err := os.Executable(); err == nil && exePath != "" {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, clean),
			filepath.Join(exeDir, "..", clean),
		)
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return filepath.Clean(relPath)
}

func handleAnswer(c tele.Context) error {
	if c.Chat().Type == tele.ChatPrivate {
		return nil
	}
	if !isContextAuthorized(c) {
		logAccessDenied(c, "callback")
		return nil
	}

	// kvID is combination of user id and chat id
	kvID := fmt.Sprintf("%v-%v", c.Callback().Sender.ID, c.Chat().ID)

	messageID := c.Callback().Message.ID
	answer := strings.TrimSpace(c.Callback().Data)
	answer = strings.Split(answer, "|")[0]

	status := captcha.JoinStatus{}
	if data, found := db.Get(kvID); !found {
		c.Respond(&tele.CallbackResponse{Text: "This challenge is not for you."})
		log.Printf("Answer rejected (missing challenge) chat_id=%d user_id=%d", c.Chat().ID, c.Callback().Sender.ID)
		return nil
	} else {
		status = data.(captcha.JoinStatus)
	}

	if messageID != status.CaptchaMessage.ID {
		c.Respond(&tele.CallbackResponse{Text: "This challenge is not for you."})
		log.Printf("Answer rejected (message mismatch) chat_id=%d user_id=%d got_message_id=%d expected_message_id=%d", c.Chat().ID, c.Callback().Sender.ID, messageID, status.CaptchaMessage.ID)
		return nil
	}

	correct, expected := isNextCaptchaAnswer(status, answer)
	if correct {
		status.SolvedCaptcha++
	} else {
		status.FailCaptcha++
		db.Update(kvID, status)
		log.Printf(
			"Answer rejected (wrong sequence) chat_id=%d user_id=%d got=%q expected=%q solved=%d total=%d",
			c.Chat().ID,
			c.Callback().Sender.ID,
			answer,
			expected,
			status.SolvedCaptcha,
			len(status.CaptchaAnswer),
		)

		if status.FailCaptcha >= cfg.Captcha.MaxFailures {
			db.Delete(kvID)
			c.Respond(&tele.CallbackResponse{Text: "Captcha failed, you have been banned, please contact admin with your another account.", ShowAlert: true})
			if err := bot.Delete(&status.CaptchaMessage); err != nil {
				log.Printf("warn: failed to delete failed captcha message chat_id=%d user_id=%d err=%v", c.Chat().ID, c.Sender().ID, err)
			}
			if err := bot.Ban(c.Chat(), &tele.ChatMember{User: c.Sender()}, false); err != nil {
				log.Printf("warn: failed to ban failed captcha user chat_id=%d user_id=%d err=%v", c.Chat().ID, c.Sender().ID, err)
			}

			mention := fmt.Sprintf(`[%v](tg://user?id=%v)`, status.UserFullName, status.UserID)
			msg := "Captcha failed, %v has been banned, please contact administrator if %v are real human with non-automated account"
			msg += "\n\n this message will automatically removed in %s..."
			msg = fmt.Sprintf(msg, mention, mention, humanizeDuration(cfg.Captcha.FailureNoticeTTL))
			msgr, err := sendWithConfiguredTopic(status.CaptchaMessage.Chat, msg, tele.ModeMarkdown, nil)
			if err == nil {
				go func(msgr *tele.Message) {
					time.Sleep(cfg.Captcha.FailureNoticeTTL)
					if err := bot.Delete(msgr); err != nil {
						log.Printf("warn: failed to delete failure notice message chat_id=%d user_id=%d err=%v", c.Chat().ID, c.Sender().ID, err)
					}
				}(msgr)
			} else {
				log.Printf("warn: failed to send failure notice chat_id=%d user_id=%d err=%v", c.Chat().ID, c.Sender().ID, err)
			}
			log.Printf("Captcha failed chat_id=%d user_id=%d solved=%d failed=%d", c.Chat().ID, c.Sender().ID, status.SolvedCaptcha, status.FailCaptcha)
			return nil
		}

		challenge, err := buildCaptchaChallenge(captchaAnswerCount, captchaDecoyCount)
		if err != nil {
			log.Printf("error: failed to regenerate captcha challenge chat_id=%d user_id=%d err=%v", c.Chat().ID, c.Sender().ID, err)
			c.Respond(&tele.CallbackResponse{Text: "Wrong sequence. Please continue with the current puzzle.", ShowAlert: true})
			return nil
		}

		file := tele.FromReader(bytes.NewReader(challenge.ImageBytes))
		photo := &tele.Photo{File: file}
		photo.Caption = genCaption(c.Sender())

		newMsg, err := sendWithConfiguredTopic(c.Chat(), photo, tele.ModeMarkdown, challenge.Markup)
		if err != nil {
			log.Printf("error: failed to send regenerated captcha challenge chat_id=%d user_id=%d err=%v", c.Chat().ID, c.Sender().ID, err)
			c.Respond(&tele.CallbackResponse{Text: "Wrong sequence. Please continue with the current puzzle.", ShowAlert: true})
			return nil
		}

		oldMessage := status.CaptchaMessage
		applyCaptchaChallenge(&status, challenge, *newMsg)
		db.Update(kvID, status)
		if err := bot.Delete(&oldMessage); err != nil {
			log.Printf("warn: failed to delete previous captcha message chat_id=%d user_id=%d message_id=%d err=%v", c.Chat().ID, c.Sender().ID, oldMessage.ID, err)
		}
		c.Respond(&tele.CallbackResponse{Text: "Wrong sequence. A new puzzle has been generated.", ShowAlert: true})
		log.Printf("Captcha regenerated chat_id=%d user_id=%d old_message_id=%d new_message_id=%d failed=%d", c.Chat().ID, c.Sender().ID, oldMessage.ID, newMsg.ID, status.FailCaptcha)
		return nil
	}

	newButtons := make([]tele.InlineButton, 0)
	for _, button := range status.Buttons {
		if button.Unique == answer {
			if correct {
				button.Text = "✅"
			} else {
				button.Text = "❌"
			}
		}
		newButtons = append(newButtons, button)
	}
	status.Buttons = newButtons

	db.Update(kvID, status)

	updateBtn := captchaMarkupFromButtons(newButtons)
	if len(newButtons) == 0 {
		log.Printf("warn: no captcha buttons available for update chat_id=%d user_id=%d", c.Chat().ID, c.Sender().ID)
		return nil
	}
	if _, err := bot.Edit(c.Callback(), updateBtn); err != nil {
		log.Printf("warn: failed to update captcha keyboard chat_id=%d user_id=%d err=%v", c.Chat().ID, c.Sender().ID, err)
	}

	if status.SolvedCaptcha >= len(status.CaptchaAnswer) {
		db.Delete(kvID)
		c.Respond(&tele.CallbackResponse{Text: "Successfully joined.", ShowAlert: true})
		if err := bot.Delete(&status.CaptchaMessage); err != nil {
			log.Printf("warn: failed to delete solved captcha message chat_id=%d user_id=%d err=%v", c.Chat().ID, c.Sender().ID, err)
		}

		chatMember, err := bot.ChatMemberOf(c.Chat(), c.Sender())
		if err != nil {
			log.Printf("warn: failed to load member state for unrestrict chat_id=%d user_id=%d err=%v", c.Chat().ID, c.Sender().ID, err)
			return nil
		}
		chatMember.Rights = tele.NoRestrictions()
		if err := bot.Restrict(c.Chat(), chatMember); err != nil {
			log.Printf("warn: failed to restore user permissions chat_id=%d user_id=%d err=%v", c.Chat().ID, c.Sender().ID, err)
		}
		log.Printf("Captcha solved chat_id=%d user_id=%d solved=%d failed=%d", c.Chat().ID, c.Sender().ID, status.SolvedCaptcha, status.FailCaptcha)

		return nil
	}

	return nil
}

func isNextCaptchaAnswer(status captcha.JoinStatus, answer string) (bool, string) {
	if status.SolvedCaptcha < 0 || status.SolvedCaptcha >= len(status.CaptchaAnswer) {
		return false, ""
	}
	expected := strings.TrimSpace(status.CaptchaAnswer[status.SolvedCaptcha])
	return answer == expected, expected
}

func onEvicted(key string, value interface{}) {
	if val, ok := value.(captcha.JoinStatus); ok {
		log.Printf("Captcha expired chat_id=%d user_id=%d", val.ChatID, val.UserID)
		mention := fmt.Sprintf(`[%v](tg://user?id=%v)`, val.UserFullName, val.UserID)
		msg := "Captcha failed, %v has been banned, please contact administrator if %v are real human with non-automated account"
		msg += "\n\n this message will automatically removed in %s..."
		msg = fmt.Sprintf(msg, mention, mention, humanizeDuration(cfg.Captcha.FailureNoticeTTL))
		msgr, err := sendWithConfiguredTopic(val.CaptchaMessage.Chat, msg, tele.ModeMarkdown, nil)
		if err == nil {
			go func(msgr *tele.Message) {
				time.Sleep(cfg.Captcha.FailureNoticeTTL)
				if err := bot.Delete(msgr); err != nil {
					log.Printf("warn: failed to delete eviction notice message chat_id=%d user_id=%d err=%v", val.ChatID, val.UserID, err)
				}
			}(msgr)
		} else {
			log.Printf("warn: failed to send eviction notice chat_id=%d user_id=%d err=%v", val.ChatID, val.UserID, err)
		}

		if err := bot.Delete(&val.CaptchaMessage); err != nil {
			log.Printf("warn: failed to delete expired captcha message chat_id=%d user_id=%d err=%v", val.ChatID, val.UserID, err)
		}
		if err := bot.Ban(val.CaptchaMessage.Chat, &tele.ChatMember{User: &tele.User{ID: val.UserID}}, false); err != nil {
			log.Printf("warn: failed to ban user after captcha expiry chat_id=%d user_id=%d err=%v", val.ChatID, val.UserID, err)
		}
	}
}
