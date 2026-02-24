package main

import (
	"bytes"
	"fmt"
	"image/jpeg"
	"log"
	"math/rand"
	"strings"
	"time"

	gim "github.com/codenoid/goimagemerge"
	tele "gopkg.in/telebot.v3"
)

func onPing(c tele.Context) error {
	if !isSenderAllowed(c) {
		logAccessDenied(c, "ping_sender_not_allowed")
		return nil
	}
	if c.Chat() == nil {
		log.Printf("warn: ping skipped reason=missing_chat_context")
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
		cfg.Bot.TopicThreadID,
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
	if !isSenderAllowedOrAdmin(c) {
		logAccessDenied(c, "testcaptcha_sender_not_allowed")
		return nil
	}
	return onJoin(c)
}

func onJoin(c tele.Context) error {
	if c.Chat().Type == tele.ChatPrivate {
		return nil
	}
	if !isContextAuthorized(c) {
		logAccessDenied(c, "join")
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

	const answerCount = 4
	const decoyCount = 6
	const challengeCount = answerCount + decoyCount

	emojiKeys := make([]string, 0, len(emojis))
	for key := range emojis {
		emojiKeys = append(emojiKeys, key)
	}
	if len(emojiKeys) < challengeCount {
		log.Printf("error: captcha generation aborted chat_id=%d user_id=%d reason=insufficient_emoji_pool available=%d required=%d", c.Chat().ID, c.Sender().ID, len(emojiKeys), challengeCount)
		return nil
	}

	rand.Shuffle(len(emojiKeys), func(i, j int) {
		emojiKeys[i], emojiKeys[j] = emojiKeys[j], emojiKeys[i]
	})

	answerKeys := append([]string(nil), emojiKeys[:answerCount]...)
	challengeKeys := append([]string(nil), emojiKeys[:challengeCount]...)
	rand.Shuffle(len(challengeKeys), func(i, j int) {
		challengeKeys[i], challengeKeys[j] = challengeKeys[j], challengeKeys[i]
	})

	// generate image
	captchaGrids := make([]*gim.Grid, 0)
	i := 0
	for _, key := range answerKeys {
		x := 10
		if i > 0 {
			x = i * 100
		}
		captchaGrids = append(captchaGrids, &gim.Grid{
			ImageFilePath: fmt.Sprintf("./assets/image/emoji/%v.png", key),
			OffsetX:       x, OffsetY: 120,
			Rotate: float64(rand.Intn(200-0) + 0),
		})
		i++
	}

	grids := []*gim.Grid{
		{
			ImageFilePath: "./gopherbg.jpg",
			Grids:         captchaGrids,
		},
	}

	rgba, _ := gim.New(grids, 1, 1).Merge()

	var img bytes.Buffer
	jpeg.Encode(&img, rgba, &jpeg.Options{Quality: 100})

	// generate keyboard and send image
	menu := &tele.ReplyMarkup{ResizeKeyboard: true}
	btn1 := make([]tele.Btn, 0)
	btn2 := make([]tele.Btn, 0)
	buttons := make([]tele.InlineButton, 0)

	for i, key := range challengeKeys {
		emoji := emojis[key]
		buttons = append(buttons, tele.InlineButton{Text: emoji, Unique: key})
		if i < 5 {
			btn1 = append(btn1, menu.Data(emoji, key))
		} else {
			btn2 = append(btn2, menu.Data(emoji, key))
		}
		i++
	}

	if len(btn2) > 0 {
		menu.Inline(
			menu.Row(btn1...),
			menu.Row(btn2...),
		)
	} else {
		menu.Inline(menu.Row(btn1...))
	}

	file := tele.FromReader(bytes.NewReader(img.Bytes()))
	photo := &tele.Photo{File: file}
	photo.Caption = genCaption(c.Sender())

	msg, err := sendWithConfiguredTopic(c.Chat(), photo, tele.ModeMarkdown, menu)
	if err == nil {
		captchaAnswer := make([]string, 0, len(answerKeys))
		for _, key := range answerKeys {
			captchaAnswer = append(captchaAnswer, strings.TrimSpace(key))
		}
		status := JoinStatus{
			UserID:         c.Sender().ID,
			CaptchaAnswer:  captchaAnswer,
			ChatID:         c.Chat().ID,
			CaptchaMessage: *msg,
			Buttons:        buttons,
		}

		status.UserFullName = c.Sender().FirstName + " " + c.Sender().LastName
		status.UserFullName = sanitizeName(status.UserFullName)

		db.Set(kvID, status, cfg.Captcha.Expiration)
		log.Printf(
			"Captcha issued chat_id=%d user_id=%d challenge_message_id=%d answer_count=%d topic_thread_id=%d",
			c.Chat().ID,
			c.Sender().ID,
			msg.ID,
			len(captchaAnswer),
			cfg.Bot.TopicThreadID,
		)

		chatMember, err := bot.ChatMemberOf(c.Chat(), c.Sender())
		if err != nil {
			log.Printf("warn: failed to load member state for restriction chat_id=%d user_id=%d err=%v", c.Chat().ID, c.Sender().ID, err)
			return nil
		}
		chatMember.RestrictedUntil = time.Now().Add(cfg.Captcha.Expiration).Unix()
		if err := bot.Restrict(c.Chat(), chatMember); err != nil {
			log.Printf("warn: failed to restrict user chat_id=%d user_id=%d err=%v", c.Chat().ID, c.Sender().ID, err)
		}
	} else {
		log.Printf("error: failed to send captcha challenge chat_id=%d user_id=%d err=%v", c.Chat().ID, c.Sender().ID, err)
	}

	return nil
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

	status := JoinStatus{}
	if data, found := db.Get(kvID); !found {
		c.Respond(&tele.CallbackResponse{Text: "This challenge is not for you."})
		log.Printf("Answer rejected (missing challenge) chat_id=%d user_id=%d", c.Chat().ID, c.Callback().Sender.ID)
		return nil
	} else {
		status = data.(JoinStatus)
	}

	if messageID != status.CaptchaMessage.ID {
		c.Respond(&tele.CallbackResponse{Text: "This challenge is not for you."})
		log.Printf("Answer rejected (message mismatch) chat_id=%d user_id=%d got_message_id=%d expected_message_id=%d", c.Chat().ID, c.Callback().Sender.ID, messageID, status.CaptchaMessage.ID)
		return nil
	}

	correct, expected := isNextCaptchaAnswer(status, answer)
	if correct {
		status.SolvedCaptcha++
		db.Update(kvID, status)
	} else {
		status.FailCaptcha++
		log.Printf(
			"Answer rejected (wrong sequence) chat_id=%d user_id=%d got=%q expected=%q solved=%d total=%d",
			c.Chat().ID,
			c.Callback().Sender.ID,
			answer,
			expected,
			status.SolvedCaptcha,
			len(status.CaptchaAnswer),
		)
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

	updateBtn := &tele.ReplyMarkup{
		Selective:      true,
		InlineKeyboard: [][]tele.InlineButton{},
	}
	if len(newButtons) == 0 {
		log.Printf("warn: no captcha buttons available for update chat_id=%d user_id=%d", c.Chat().ID, c.Sender().ID)
		return nil
	}
	if len(newButtons) <= 5 {
		updateBtn.InlineKeyboard = append(updateBtn.InlineKeyboard, newButtons)
	} else {
		updateBtn.InlineKeyboard = append(updateBtn.InlineKeyboard, newButtons[:5], newButtons[5:])
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
	} else if status.FailCaptcha >= cfg.Captcha.MaxFailures {
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
	}

	return nil
}

func isNextCaptchaAnswer(status JoinStatus, answer string) (bool, string) {
	if status.SolvedCaptcha < 0 || status.SolvedCaptcha >= len(status.CaptchaAnswer) {
		return false, ""
	}
	expected := strings.TrimSpace(status.CaptchaAnswer[status.SolvedCaptcha])
	return answer == expected, expected
}

func onEvicted(key string, value interface{}) {
	if val, ok := value.(JoinStatus); ok {
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
