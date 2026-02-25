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

var errCaptchaSendTimeout = errors.New("captcha challenge send timeout")

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

	args := c.Args()
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		if sendErr := c.Send("Usage: `/testcaptcha @username` as a reply to that user's message.", tele.ModeMarkdown); sendErr != nil {
			log.Printf("warn: failed to send testcaptcha usage chat_id=%d actor_user_id=%d err=%v", c.Chat().ID, c.Sender().ID, sendErr)
		}
		return nil
	}

	targetUser, err := resolveTestCaptchaTarget(c.Sender(), c.Message(), args[0])
	if err != nil {
		log.Printf("warn: testcaptcha target resolution failed chat_id=%d actor_user_id=%d arg=%q err=%v", c.Chat().ID, c.Sender().ID, args[0], err)
		if sendErr := c.Send("Unable to resolve target user. Use `/testcaptcha @username` as a reply to that user's message.", tele.ModeMarkdown); sendErr != nil {
			log.Printf("warn: failed to send testcaptcha resolution error chat_id=%d actor_user_id=%d err=%v", c.Chat().ID, c.Sender().ID, sendErr)
		}
		return nil
	}

	log.Printf("Manual captcha trigger chat_id=%d actor_user_id=%d target_user_id=%d target_username=%q", c.Chat().ID, c.Sender().ID, targetUser.ID, targetUser.Username)
	return issueCaptchaChallenge(c, targetUser, true, true)
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

	return issueCaptchaChallenge(c, c.Sender(), true, false)
}

func issueCaptchaChallenge(c tele.Context, targetUser *tele.User, deleteTriggerMessage bool, manualChallenge bool) error {
	if c == nil || c.Chat() == nil || targetUser == nil {
		return nil
	}

	// delete any incoming message before challenge solved
	if deleteTriggerMessage && c.Message() != nil {
		if err := bot.Delete(c.Message()); err != nil {
			log.Printf("warn: failed to delete trigger message chat_id=%d user_id=%d err=%v", c.Chat().ID, targetUser.ID, err)
		}
	}

	// kvID is combination of user id and chat id
	kvID := fmt.Sprintf("%v-%v", targetUser.ID, c.Chat().ID)

	// skip captcha-generation if data still exist
	if _, found := db.Get(kvID); found {
		log.Printf("Captcha already pending chat_id=%d user_id=%d", c.Chat().ID, targetUser.ID)
		if manualChallenge {
			mention := markdownMention(targetUser)
			if err := c.Send(fmt.Sprintf("Captcha test already in progress for %s.", mention), tele.ModeMarkdown); err != nil {
				log.Printf("warn: failed to send duplicate manual captcha notice chat_id=%d user_id=%d err=%v", c.Chat().ID, targetUser.ID, err)
			}
		}
		return nil
	}

	var chatMember *tele.ChatMember
	var originalMember *tele.ChatMember
	if !manualChallenge {
		member, err := bot.ChatMemberOf(c.Chat(), targetUser)
		if err != nil {
			log.Printf("warn: failed to load member state for restriction chat_id=%d user_id=%d err=%v", c.Chat().ID, targetUser.ID, err)
			return nil
		}
		chatMember = member
		original := *chatMember
		originalMember = &original

		applyCaptchaRestriction(chatMember, cfg.Captcha.Expiration)
		if err := bot.Restrict(c.Chat(), chatMember); err != nil {
			log.Printf("warn: failed to restrict user chat_id=%d user_id=%d err=%v", c.Chat().ID, targetUser.ID, err)
			if c.Sender() != nil && targetUser.ID != c.Sender().ID {
				if sendErr := c.Send("Failed to restrict target user. Ensure the target is not an admin and bot has restrict permissions."); sendErr != nil {
					log.Printf("warn: failed to send restrict failure notice chat_id=%d actor_user_id=%d target_user_id=%d err=%v", c.Chat().ID, c.Sender().ID, targetUser.ID, sendErr)
				}
			}
			return nil
		}
		log.Printf("User restricted pending captcha chat_id=%d user_id=%d until=%d", c.Chat().ID, targetUser.ID, chatMember.RestrictedUntil)
	}

	challenge, err := buildCaptchaChallenge(captchaAnswerCount, captchaDecoyCount)
	if err != nil {
		log.Printf("error: captcha generation failed chat_id=%d user_id=%d err=%v", c.Chat().ID, targetUser.ID, err)
		if !manualChallenge {
			restoreUserRestriction(c.Chat(), targetUser, originalMember, "captcha_generation_failed")
		}
		return nil
	}

	msg, err := sendCaptchaChallenge(c.Chat(), challenge.ImageBytes, genCaption(targetUser), challenge.Markup)
	if err != nil {
		if errors.Is(err, errCaptchaSendTimeout) {
			// Timeout is delivery-uncertain: keep challenge state for callback matching.
			if !manualChallenge {
				applyCaptchaRestriction(chatMember, cfg.Captcha.Expiration)
				if restrictErr := bot.Restrict(c.Chat(), chatMember); restrictErr != nil {
					log.Printf("warn: failed to extend user restriction after timeout chat_id=%d user_id=%d err=%v", c.Chat().ID, targetUser.ID, restrictErr)
				}
			}

			unknownMessage := tele.Message{Chat: c.Chat()}
			status := newJoinStatus(targetUser, c.Chat(), challenge, unknownMessage, manualChallenge)
			db.Set(kvID, status, cfg.Captcha.Expiration)
			if manualChallenge {
				log.Printf(
					"warn: manual captcha delivery uncertain chat_id=%d user_id=%d challenge_message_id=unknown action=wait_for_callback",
					c.Chat().ID,
					targetUser.ID,
				)
			} else {
				log.Printf(
					"warn: captcha delivery uncertain chat_id=%d user_id=%d challenge_message_id=unknown action=keep_restricted_and_wait_for_callback",
					c.Chat().ID,
					targetUser.ID,
				)
			}
			return nil
		}

		log.Printf("error: failed to send captcha challenge chat_id=%d user_id=%d err=%v", c.Chat().ID, targetUser.ID, err)
		if !manualChallenge {
			restoreUserRestriction(c.Chat(), targetUser, originalMember, "captcha_send_failed")
		}
		return nil
	}

	if !manualChallenge {
		// Refresh restriction window after successful challenge delivery so
		// expiration starts from when user can actually solve the captcha.
		applyCaptchaRestriction(chatMember, cfg.Captcha.Expiration)
		if err := bot.Restrict(c.Chat(), chatMember); err != nil {
			log.Printf("warn: failed to refresh user restriction window chat_id=%d user_id=%d err=%v", c.Chat().ID, targetUser.ID, err)
			if err := bot.Delete(msg); err != nil {
				log.Printf("warn: failed to delete captcha after restriction refresh failure chat_id=%d user_id=%d message_id=%d err=%v", c.Chat().ID, targetUser.ID, msg.ID, err)
			}
			restoreUserRestriction(c.Chat(), targetUser, originalMember, "captcha_restriction_refresh_failed")
			return nil
		}
	}

	status := newJoinStatus(targetUser, c.Chat(), challenge, *msg, manualChallenge)
	db.Set(kvID, status, cfg.Captcha.Expiration)
	log.Printf(
		"Captcha issued chat_id=%d user_id=%d challenge_message_id=%d answer_count=%d topic_thread_id=%d",
		c.Chat().ID,
		targetUser.ID,
		msg.ID,
		len(status.CaptchaAnswer),
		topicThreadIDForChat(c.Chat()),
	)

	return nil
}

func resolveTestCaptchaTarget(sender *tele.User, message *tele.Message, rawArg string) (*tele.User, error) {
	if sender == nil || message == nil {
		return nil, fmt.Errorf("missing command context")
	}

	arg := strings.TrimSpace(rawArg)
	if arg == "" {
		return nil, fmt.Errorf("target argument is empty")
	}
	if !strings.HasPrefix(arg, "@") {
		return nil, fmt.Errorf("target must be a username like @example")
	}

	expectedUsername := strings.ToLower(strings.TrimPrefix(arg, "@"))
	if expectedUsername == "" {
		return nil, fmt.Errorf("target username is empty")
	}

	if senderUsername := strings.ToLower(strings.TrimSpace(sender.Username)); senderUsername == expectedUsername {
		return sender, nil
	}

	reply := message.ReplyTo
	if reply == nil || reply.Sender == nil {
		return nil, fmt.Errorf("username resolution requires replying to the target user's message")
	}

	replyUsername := strings.ToLower(strings.TrimSpace(reply.Sender.Username))
	if replyUsername == "" {
		return nil, fmt.Errorf("reply target has no public username")
	}
	if replyUsername != expectedUsername {
		return nil, fmt.Errorf("reply target username mismatch expected=@%s got=@%s", expectedUsername, replyUsername)
	}

	return reply.Sender, nil
}

func newJoinStatus(user *tele.User, chat *tele.Chat, challenge captchaChallenge, message tele.Message, manualChallenge bool) captcha.JoinStatus {
	status := captcha.JoinStatus{}
	if user != nil {
		status.UserID = user.ID
		status.UserFullName = sanitizeName(user.FirstName + " " + user.LastName)
	}
	status.ManualChallenge = manualChallenge
	if chat != nil {
		status.ChatID = chat.ID
	}
	applyCaptchaChallenge(&status, challenge, message)
	return status
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

func shouldBanOnCaptchaFailure(status captcha.JoinStatus) bool {
	return !status.ManualChallenge
}

func markdownMention(user *tele.User) string {
	if user == nil {
		return "[user](tg://user?id=0)"
	}
	displayName := strings.TrimSpace(user.FirstName)
	if displayName == "" {
		displayName = "user"
	}
	displayName = escapeTelegramMarkdown(displayName)
	return fmt.Sprintf(`[%v](tg://user?id=%v)`, displayName, user.ID)
}

func captchaFailureUserMention(status captcha.JoinStatus) string {
	displayName := strings.TrimSpace(status.UserFullName)
	if displayName == "" {
		displayName = "user"
	}
	displayName = escapeTelegramMarkdown(displayName)
	return fmt.Sprintf(`[%v](tg://user?id=%v)`, displayName, status.UserID)
}

func captchaFailureNoticeText(status captcha.JoinStatus, banned bool, failureNoticeTTL time.Duration) string {
	mention := captchaFailureUserMention(status)
	if banned {
		msg := "Captcha failed, %v has been banned, please contact administrator if %v are real human with non-automated account"
		msg += "\n\n this message will automatically removed in %s..."
		return fmt.Sprintf(msg, mention, mention, humanizeDuration(failureNoticeTTL))
	}
	return fmt.Sprintf("%v captcha failed.", mention)
}

func captchaSuccessCallbackText(status captcha.JoinStatus) string {
	if status.ManualChallenge {
		return "Manual test captcha completed successfully."
	}
	return "Successfully joined."
}

func captchaTimeoutNoticeText(status captcha.JoinStatus) string {
	return fmt.Sprintf("Captcha timeout, %v did not resolve the challenge in time.", captchaFailureUserMention(status))
}

func sendCaptchaFailureNotice(status captcha.JoinStatus, targetChat *tele.Chat, banned bool) {
	if targetChat == nil {
		log.Printf("warn: failed to send captcha failure notice reason=missing_target_chat user_id=%d", status.UserID)
		return
	}

	msg := captchaFailureNoticeText(status, banned, cfg.Captcha.FailureNoticeTTL)
	msgr, err := sendWithConfiguredTopic(targetChat, msg, tele.ModeMarkdown, nil)
	if err != nil {
		log.Printf("warn: failed to send captcha failure notice chat_id=%d user_id=%d banned=%t err=%v", targetChat.ID, status.UserID, banned, err)
		return
	}

	if !banned {
		return
	}

	go func(msgr *tele.Message, chatID int64, userID int64) {
		time.Sleep(cfg.Captcha.FailureNoticeTTL)
		if err := bot.Delete(msgr); err != nil {
			log.Printf("warn: failed to delete failure notice message chat_id=%d user_id=%d err=%v", chatID, userID, err)
		}
	}(msgr, targetChat.ID, status.UserID)
}

func sendCaptchaTimeoutNotice(status captcha.JoinStatus, targetChat *tele.Chat) {
	if targetChat == nil {
		log.Printf("warn: failed to send captcha timeout notice reason=missing_target_chat user_id=%d", status.UserID)
		return
	}

	msg := captchaTimeoutNoticeText(status)
	if _, err := sendWithConfiguredTopic(targetChat, msg, tele.ModeMarkdown, nil); err != nil {
		log.Printf("warn: failed to send captcha timeout notice chat_id=%d user_id=%d err=%v", targetChat.ID, status.UserID, err)
	}
}

func sendCaptchaChallenge(chat *tele.Chat, imageBytes []byte, caption string, markup *tele.ReplyMarkup) (*tele.Message, error) {
	file := tele.FromReader(bytes.NewReader(imageBytes))
	photo := &tele.Photo{File: file}
	photo.Caption = caption

	msg, err := sendWithConfiguredTopic(chat, photo, tele.ModeMarkdown, markup)
	if err == nil {
		return msg, nil
	}

	if isTimeoutLikeError(err) {
		return nil, fmt.Errorf("%w: %v", errCaptchaSendTimeout, err)
	}

	return nil, err
}

func bindCaptchaMessageIfUnset(status *captcha.JoinStatus, message *tele.Message) bool {
	if status == nil || message == nil {
		return false
	}
	if status.CaptchaMessage.ID != 0 {
		return false
	}
	status.CaptchaMessage = *message
	return true
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

func handleAnswer(c tele.Context) error {
	if c == nil || c.Chat() == nil || c.Callback() == nil || c.Callback().Sender == nil || c.Callback().Message == nil {
		log.Printf(
			"warn: callback skipped reason=missing_callback_context has_context=%t has_chat=%t has_callback=%t has_sender=%t has_message=%t",
			c != nil,
			c != nil && c.Chat() != nil,
			c != nil && c.Callback() != nil,
			c != nil && c.Callback() != nil && c.Callback().Sender != nil,
			c != nil && c.Callback() != nil && c.Callback().Message != nil,
		)
		return nil
	}

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

	if bindCaptchaMessageIfUnset(&status, c.Callback().Message) {
		if err := db.Update(kvID, status); err != nil {
			log.Printf("warn: failed to persist captcha message binding chat_id=%d user_id=%d message_id=%d err=%v", c.Chat().ID, c.Callback().Sender.ID, messageID, err)
		}
		log.Printf("Captcha message bound chat_id=%d user_id=%d message_id=%d", c.Chat().ID, c.Callback().Sender.ID, messageID)
	} else if messageID != status.CaptchaMessage.ID {
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
			targetChat := status.CaptchaMessage.Chat
			if targetChat == nil {
				targetChat = c.Chat()
			}

			if status.CaptchaMessage.ID > 0 {
				if err := bot.Delete(&status.CaptchaMessage); err != nil {
					log.Printf("warn: failed to delete failed captcha message chat_id=%d user_id=%d err=%v", c.Chat().ID, c.Sender().ID, err)
				}
			}

			if !shouldBanOnCaptchaFailure(status) {
				c.Respond(&tele.CallbackResponse{Text: "Captcha failed.", ShowAlert: true})
				sendCaptchaFailureNotice(status, targetChat, false)
				log.Printf("Manual captcha failed chat_id=%d user_id=%d solved=%d failed=%d", c.Chat().ID, status.UserID, status.SolvedCaptcha, status.FailCaptcha)
				return nil
			}

			c.Respond(&tele.CallbackResponse{Text: "Captcha failed, you have been banned, please contact admin with your another account.", ShowAlert: true})
			if err := bot.Ban(targetChat, &tele.ChatMember{User: &tele.User{ID: status.UserID}}, false); err != nil {
				log.Printf("warn: failed to ban failed captcha user chat_id=%d user_id=%d err=%v", c.Chat().ID, status.UserID, err)
			}
			sendCaptchaFailureNotice(status, targetChat, true)
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
		if oldMessage.ID > 0 {
			if err := bot.Delete(&oldMessage); err != nil {
				log.Printf("warn: failed to delete previous captcha message chat_id=%d user_id=%d message_id=%d err=%v", c.Chat().ID, c.Sender().ID, oldMessage.ID, err)
			}
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
		c.Respond(&tele.CallbackResponse{Text: captchaSuccessCallbackText(status), ShowAlert: true})
		if status.CaptchaMessage.ID > 0 {
			if err := bot.Delete(&status.CaptchaMessage); err != nil {
				log.Printf("warn: failed to delete solved captcha message chat_id=%d user_id=%d err=%v", c.Chat().ID, c.Sender().ID, err)
			}
		}

		if status.ManualChallenge {
			log.Printf("Manual captcha solved chat_id=%d user_id=%d solved=%d failed=%d", c.Chat().ID, c.Sender().ID, status.SolvedCaptcha, status.FailCaptcha)
			return nil
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
		targetChat := val.CaptchaMessage.Chat
		if targetChat == nil {
			targetChat = &tele.Chat{ID: val.ChatID}
		}
		if val.CaptchaMessage.ID > 0 {
			if err := bot.Delete(&val.CaptchaMessage); err != nil {
				log.Printf("warn: failed to delete expired captcha message chat_id=%d user_id=%d err=%v", val.ChatID, val.UserID, err)
			}
		}

		if !shouldBanOnCaptchaFailure(val) {
			sendCaptchaTimeoutNotice(val, targetChat)
			log.Printf("Manual captcha expired chat_id=%d user_id=%d", val.ChatID, val.UserID)
			return
		}

		sendCaptchaFailureNotice(val, targetChat, true)
		if err := bot.Ban(targetChat, &tele.ChatMember{User: &tele.User{ID: val.UserID}}, false); err != nil {
			log.Printf("warn: failed to ban user after captcha expiry chat_id=%d user_id=%d err=%v", val.ChatID, val.UserID, err)
		}
	}
}
