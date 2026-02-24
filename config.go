package main

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

const defaultConfigPath = "config.yaml"

type runtimeConfig struct {
	Bot     botConfig     `yaml:"bot"`
	Captcha captchaConfig `yaml:"captcha"`
}

type botConfig struct {
	Token          string             `yaml:"token"`
	PollTimeout    time.Duration      `yaml:"poll_timeout"`
	TopicLink      string             `yaml:"topic_link"`
	Public         bool               `yaml:"public"`
	AllowedUserIDs []int64            `yaml:"allowed_user_ids"`
	allowedUsers   map[int64]struct{} `yaml:"-"`
	TopicThreadID  int                `yaml:"-"`
}

type captchaConfig struct {
	Expiration       time.Duration `yaml:"expiration"`
	CleanupInterval  time.Duration `yaml:"cleanup_interval"`
	MaxFailures      int           `yaml:"max_failures"`
	FailureNoticeTTL time.Duration `yaml:"failure_notice_ttl"`
}

func defaultRuntimeConfig() runtimeConfig {
	return runtimeConfig{
		Bot: botConfig{
			PollTimeout: 10 * time.Second,
			Public:      true,
		},
		Captcha: captchaConfig{
			Expiration:       1 * time.Minute,
			CleanupInterval:  5 * time.Second,
			MaxFailures:      2,
			FailureNoticeTTL: 15 * time.Second,
		},
	}
}

func loadConfig(path string) (runtimeConfig, error) {
	if strings.TrimSpace(path) == "" {
		path = defaultConfigPath
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return runtimeConfig{}, fmt.Errorf("read config file %q: %w", path, err)
	}

	cfg := defaultRuntimeConfig()
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return runtimeConfig{}, fmt.Errorf("decode YAML config file %q: %w", path, err)
	}

	if err := cfg.validate(); err != nil {
		return runtimeConfig{}, fmt.Errorf("invalid config file %q: %w", path, err)
	}

	return cfg, nil
}

func (c *runtimeConfig) validate() error {
	if strings.TrimSpace(c.Bot.Token) == "" {
		return fmt.Errorf("bot.token is required")
	}
	if c.Bot.PollTimeout <= 0 {
		return fmt.Errorf("bot.poll_timeout must be greater than zero")
	}

	topicThreadID, err := parseTopicIDReference(c.Bot.TopicLink)
	if err != nil {
		return fmt.Errorf("bot.topic_link is invalid: %w", err)
	}
	c.Bot.TopicThreadID = topicThreadID

	allowedUsers := make(map[int64]struct{}, len(c.Bot.AllowedUserIDs))
	for _, userID := range c.Bot.AllowedUserIDs {
		if userID <= 0 {
			return fmt.Errorf("bot.allowed_user_ids must contain positive integers")
		}
		allowedUsers[userID] = struct{}{}
	}
	c.Bot.allowedUsers = allowedUsers

	if !c.Bot.Public && len(c.Bot.allowedUsers) == 0 {
		return fmt.Errorf("bot.allowed_user_ids is required when bot.public is false")
	}

	if c.Captcha.Expiration <= 0 {
		return fmt.Errorf("captcha.expiration must be greater than zero")
	}
	if c.Captcha.CleanupInterval <= 0 {
		return fmt.Errorf("captcha.cleanup_interval must be greater than zero")
	}
	if c.Captcha.MaxFailures <= 0 {
		return fmt.Errorf("captcha.max_failures must be greater than zero")
	}
	if c.Captcha.FailureNoticeTTL <= 0 {
		return fmt.Errorf("captcha.failure_notice_ttl must be greater than zero")
	}
	return nil
}

func parseTopicIDReference(ref string) (int, error) {
	cleanRef := strings.TrimSpace(ref)
	if cleanRef == "" {
		return 0, nil
	}

	parsedURL, err := url.Parse(cleanRef)
	if err != nil {
		return 0, fmt.Errorf("parse URL: %w", err)
	}
	if parsedURL.Host == "" {
		return 0, fmt.Errorf("missing host in URL")
	}

	host := strings.TrimPrefix(strings.ToLower(parsedURL.Host), "www.")
	if host != "t.me" && host != "telegram.me" {
		return 0, fmt.Errorf("unsupported host %q (expected t.me or telegram.me)", parsedURL.Host)
	}

	if threadParam := strings.TrimSpace(parsedURL.Query().Get("thread")); threadParam != "" {
		topicID, err := strconv.Atoi(threadParam)
		if err != nil || topicID <= 0 {
			return 0, fmt.Errorf("invalid thread query parameter %q", threadParam)
		}
		return topicID, nil
	}

	segments := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	numeric := make([]int, 0, len(segments))
	for _, segment := range segments {
		n, err := strconv.Atoi(segment)
		if err != nil || n <= 0 {
			continue
		}
		numeric = append(numeric, n)
	}

	if len(numeric) == 0 {
		return 0, fmt.Errorf("no numeric topic identifier found in URL path")
	}

	if len(segments) >= 3 && strings.EqualFold(segments[0], "c") {
		if len(numeric) == 1 {
			return numeric[0], nil
		}
		if len(numeric) == 2 {
			return numeric[1], nil
		}
		return numeric[len(numeric)-2], nil
	}

	// Links can have the topic in path as:
	// - /<chat-or-user>/<topic>/<message>
	// - /c/<chat>/<topic>/<message>
	// If only one numeric segment is present, use it as best-effort fallback.
	if len(numeric) >= 2 {
		return numeric[len(numeric)-2], nil
	}

	return numeric[len(numeric)-1], nil
}
