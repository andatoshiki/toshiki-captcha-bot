package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRuntimeConfigValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*runtimeConfig)
		wantErr string
	}{
		{
			name: "valid config",
		},
		{
			name: "missing bot token",
			mutate: func(cfg *runtimeConfig) {
				cfg.Bot.Token = ""
			},
			wantErr: "bot.token is required",
		},
		{
			name: "invalid poll timeout",
			mutate: func(cfg *runtimeConfig) {
				cfg.Bot.PollTimeout = 0
			},
			wantErr: "bot.poll_timeout",
		},
		{
			name: "invalid topic link host",
			mutate: func(cfg *runtimeConfig) {
				cfg.Bot.TopicLink = "https://example.com/c/1/2"
			},
			wantErr: "bot.topic_link",
		},
		{
			name: "invalid topic link thread query",
			mutate: func(cfg *runtimeConfig) {
				cfg.Bot.TopicLink = "https://t.me/c/1234567890/4?thread=abc"
			},
			wantErr: "bot.topic_link",
		},
		{
			name: "invalid expiration",
			mutate: func(cfg *runtimeConfig) {
				cfg.Captcha.Expiration = 0
			},
			wantErr: "captcha.expiration",
		},
		{
			name: "invalid cleanup interval",
			mutate: func(cfg *runtimeConfig) {
				cfg.Captcha.CleanupInterval = 0
			},
			wantErr: "captcha.cleanup_interval",
		},
		{
			name: "invalid max failures",
			mutate: func(cfg *runtimeConfig) {
				cfg.Captcha.MaxFailures = 0
			},
			wantErr: "captcha.max_failures",
		},
		{
			name: "invalid failure notice ttl",
			mutate: func(cfg *runtimeConfig) {
				cfg.Captcha.FailureNoticeTTL = 0
			},
			wantErr: "captcha.failure_notice_ttl",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := defaultRuntimeConfig()
			cfg.Bot.Token = "test-token"
			if tt.mutate != nil {
				tt.mutate(&cfg)
			}

			err := cfg.validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validate returned error: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	t.Run("applies defaults for omitted fields", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		content := "bot:\n  token: test-token\n"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write config file: %v", err)
		}

		cfg, err := loadConfig(path)
		if err != nil {
			t.Fatalf("loadConfig returned error: %v", err)
		}

		want := defaultRuntimeConfig()
		if cfg.Bot.Token != "test-token" {
			t.Fatalf("Bot.Token = %q, want %q", cfg.Bot.Token, "test-token")
		}
		if cfg.Bot.PollTimeout != want.Bot.PollTimeout {
			t.Fatalf("Bot.PollTimeout = %v, want %v", cfg.Bot.PollTimeout, want.Bot.PollTimeout)
		}
		if cfg.Bot.TopicThreadID != 0 {
			t.Fatalf("Bot.TopicThreadID = %d, want 0", cfg.Bot.TopicThreadID)
		}
		if cfg.Captcha.Expiration != want.Captcha.Expiration {
			t.Fatalf("Captcha.Expiration = %v, want %v", cfg.Captcha.Expiration, want.Captcha.Expiration)
		}
		if cfg.Captcha.CleanupInterval != want.Captcha.CleanupInterval {
			t.Fatalf("Captcha.CleanupInterval = %v, want %v", cfg.Captcha.CleanupInterval, want.Captcha.CleanupInterval)
		}
		if cfg.Captcha.MaxFailures != want.Captcha.MaxFailures {
			t.Fatalf("Captcha.MaxFailures = %v, want %v", cfg.Captcha.MaxFailures, want.Captcha.MaxFailures)
		}
		if cfg.Captcha.FailureNoticeTTL != want.Captcha.FailureNoticeTTL {
			t.Fatalf("Captcha.FailureNoticeTTL = %v, want %v", cfg.Captcha.FailureNoticeTTL, want.Captcha.FailureNoticeTTL)
		}
	})

	t.Run("derives topic thread id from topic link", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		content := "bot:\n  token: test-token\n  topic_link: https://t.me/c/1234567890/4/77\n"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write config file: %v", err)
		}

		cfg, err := loadConfig(path)
		if err != nil {
			t.Fatalf("loadConfig returned error: %v", err)
		}

		if cfg.Bot.TopicThreadID != 4 {
			t.Fatalf("Bot.TopicThreadID = %d, want 4", cfg.Bot.TopicThreadID)
		}
	})

	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "invalid yaml",
			content: "bot: [",
			wantErr: "decode YAML",
		},
		{
			name:    "missing token",
			content: "bot:\n  poll_timeout: 10s\n",
			wantErr: "bot.token is required",
		},
		{
			name:    "invalid duration value",
			content: "bot:\n  token: test\n  poll_timeout: nope\n",
			wantErr: "decode YAML",
		},
		{
			name:    "invalid topic link",
			content: "bot:\n  token: test\n  topic_link: https://example.com/c/1/2\n",
			wantErr: "bot.topic_link is invalid",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatalf("write config file: %v", err)
			}

			_, err := loadConfig(path)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestParseTopicIDReference(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantID  int
		wantErr string
	}{
		{
			name:   "empty string means disabled topic routing",
			input:  "",
			wantID: 0,
		},
		{
			name:   "single numeric segment fallback",
			input:  "https://t.me/c/1234567890/4",
			wantID: 4,
		},
		{
			name:   "topic and message segments",
			input:  "https://t.me/c/1234567890/4/77",
			wantID: 4,
		},
		{
			name:   "public link with thread query",
			input:  "https://t.me/my_group/123?thread=9",
			wantID: 9,
		},
		{
			name:    "invalid host",
			input:   "https://example.com/c/123/4",
			wantErr: "unsupported host",
		},
		{
			name:    "no numeric segment",
			input:   "https://t.me/c/mygroup/general",
			wantErr: "no numeric topic identifier",
		},
		{
			name:    "invalid thread query",
			input:   "https://t.me/c/1234567890/4?thread=foo",
			wantErr: "invalid thread query parameter",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseTopicIDReference(tt.input)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("parseTopicIDReference returned error: %v", err)
				}
				if got != tt.wantID {
					t.Fatalf("topic id = %d, want %d", got, tt.wantID)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestHumanizeDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   time.Duration
		want string
	}{
		{
			name: "seconds",
			in:   15 * time.Second,
			want: "15 seconds",
		},
		{
			name: "minutes",
			in:   2 * time.Minute,
			want: "2 minutes",
		},
		{
			name: "hours",
			in:   3 * time.Hour,
			want: "3 hours",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := humanizeDuration(tt.in)
			if got != tt.want {
				t.Fatalf("humanizeDuration(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
