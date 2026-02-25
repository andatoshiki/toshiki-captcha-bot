package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeConfigValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*RuntimeConfig)
		wantErr string
	}{
		{
			name: "valid config in public mode",
		},
		{
			name: "missing bot token",
			mutate: func(cfg *RuntimeConfig) {
				cfg.Bot.Token = ""
			},
			wantErr: "bot.token is required",
		},
		{
			name: "invalid poll timeout",
			mutate: func(cfg *RuntimeConfig) {
				cfg.Bot.PollTimeout = 0
			},
			wantErr: "bot.poll_timeout",
		},
		{
			name: "invalid admin user id",
			mutate: func(cfg *RuntimeConfig) {
				cfg.Bot.AdminUserIDs = []int64{0}
			},
			wantErr: "bot.admin_user_ids must contain positive integers",
		},
		{
			name: "private mode with valid groups",
			mutate: func(cfg *RuntimeConfig) {
				cfg.Bot.AdminUserIDs = []int64{1001}
				cfg.Groups = []GroupTopicConfig{
					{ID: "@somegroup", Topic: 9},
					{ID: "anothergroup", Topic: 0},
				}
			},
		},
		{
			name: "private mode invalid group username",
			mutate: func(cfg *RuntimeConfig) {
				cfg.Bot.AdminUserIDs = []int64{1001}
				cfg.Groups = []GroupTopicConfig{
					{ID: "bad group name", Topic: 1},
				}
			},
			wantErr: "groups[0].id is invalid",
		},
		{
			name: "private mode duplicate groups",
			mutate: func(cfg *RuntimeConfig) {
				cfg.Bot.AdminUserIDs = []int64{1001}
				cfg.Groups = []GroupTopicConfig{
					{ID: "@samegroup", Topic: 1},
					{ID: "samegroup", Topic: 2},
				}
			},
			wantErr: "duplicates",
		},
		{
			name: "private mode invalid topic",
			mutate: func(cfg *RuntimeConfig) {
				cfg.Bot.AdminUserIDs = []int64{1001}
				cfg.Groups = []GroupTopicConfig{
					{ID: "@somegroup", Topic: -1},
				}
			},
			wantErr: "groups[0].topic",
		},
		{
			name: "private mode requires non empty groups",
			mutate: func(cfg *RuntimeConfig) {
				cfg.Bot.AdminUserIDs = []int64{1001}
				cfg.Groups = nil
			},
			wantErr: "groups must contain at least one public group when bot.admin_user_ids is set",
		},
		{
			name: "public mode discards groups config",
			mutate: func(cfg *RuntimeConfig) {
				cfg.Groups = []GroupTopicConfig{
					{ID: "invalid group id with spaces", Topic: 99},
				}
			},
		},
		{
			name: "invalid expiration",
			mutate: func(cfg *RuntimeConfig) {
				cfg.Captcha.Expiration = 0
			},
			wantErr: "captcha.expiration",
		},
		{
			name: "invalid cleanup interval",
			mutate: func(cfg *RuntimeConfig) {
				cfg.Captcha.CleanupInterval = 0
			},
			wantErr: "captcha.cleanup_interval",
		},
		{
			name: "invalid max failures",
			mutate: func(cfg *RuntimeConfig) {
				cfg.Captcha.MaxFailures = 0
			},
			wantErr: "captcha.max_failures",
		},
		{
			name: "invalid failure notice ttl",
			mutate: func(cfg *RuntimeConfig) {
				cfg.Captcha.FailureNoticeTTL = 0
			},
			wantErr: "captcha.failure_notice_ttl",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := DefaultRuntimeConfig()
			cfg.Bot.Token = "test-token"
			if tt.mutate != nil {
				tt.mutate(&cfg)
			}

			err := cfg.Validate()
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

		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load returned error: %v", err)
		}

		want := DefaultRuntimeConfig()
		if cfg.Bot.Token != "test-token" {
			t.Fatalf("Bot.Token = %q, want %q", cfg.Bot.Token, "test-token")
		}
		if cfg.Bot.PollTimeout != want.Bot.PollTimeout {
			t.Fatalf("Bot.PollTimeout = %v, want %v", cfg.Bot.PollTimeout, want.Bot.PollTimeout)
		}
		if len(cfg.Bot.AdminUserIDs) != 0 {
			t.Fatalf("Bot.AdminUserIDs length = %d, want 0", len(cfg.Bot.AdminUserIDs))
		}
		if len(cfg.Bot.adminUsers) != 0 {
			t.Fatalf("Bot.adminUsers length = %d, want 0", len(cfg.Bot.adminUsers))
		}
		if !cfg.IsPublicMode() {
			t.Fatalf("IsPublicMode = false, want true")
		}
		if len(cfg.Groups) != 0 {
			t.Fatalf("Groups length = %d, want 0", len(cfg.Groups))
		}
		if len(cfg.groupTopics) != 0 {
			t.Fatalf("groupTopics length = %d, want 0", len(cfg.groupTopics))
		}
		if len(cfg.groupAllow) != 0 {
			t.Fatalf("groupAllow length = %d, want 0", len(cfg.groupAllow))
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

	t.Run("private mode with groups and topics", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		content := strings.Join([]string{
			"bot:",
			"  token: test-token",
			"  admin_user_ids: [1001, 1002]",
			"groups:",
			"  - id: \"@SomePublicGroup\"",
			"    topic: 4",
			"  - id: anothergroup",
			"captcha:",
			"  expiration: 1m",
			"  cleanup_interval: 5s",
			"  max_failures: 2",
			"  failure_notice_ttl: 15s",
			"",
		}, "\n")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write config file: %v", err)
		}

		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load returned error: %v", err)
		}

		if cfg.IsPublicMode() {
			t.Fatalf("IsPublicMode = true, want false")
		}
		if len(cfg.Bot.adminUsers) != 2 {
			t.Fatalf("Bot.adminUsers length = %d, want 2", len(cfg.Bot.adminUsers))
		}
		if _, ok := cfg.Bot.adminUsers[1001]; !ok {
			t.Fatalf("Bot.adminUsers missing user id 1001")
		}
		if _, ok := cfg.Bot.adminUsers[1002]; !ok {
			t.Fatalf("Bot.adminUsers missing user id 1002")
		}

		if len(cfg.Groups) != 2 {
			t.Fatalf("Groups length = %d, want 2", len(cfg.Groups))
		}
		if cfg.Groups[0].ID != "@somepublicgroup" {
			t.Fatalf("Groups[0].ID = %q, want %q", cfg.Groups[0].ID, "@somepublicgroup")
		}
		if cfg.Groups[0].Topic != 4 {
			t.Fatalf("Groups[0].Topic = %d, want 4", cfg.Groups[0].Topic)
		}
		if cfg.Groups[1].ID != "@anothergroup" {
			t.Fatalf("Groups[1].ID = %q, want %q", cfg.Groups[1].ID, "@anothergroup")
		}
		if cfg.groupTopics["somepublicgroup"] != 4 {
			t.Fatalf("groupTopics[somepublicgroup] = %d, want 4", cfg.groupTopics["somepublicgroup"])
		}
		if _, ok := cfg.groupTopics["anothergroup"]; ok {
			t.Fatalf("groupTopics[anothergroup] should not be set when topic is omitted")
		}
		if _, ok := cfg.groupAllow["somepublicgroup"]; !ok {
			t.Fatalf("groupAllow missing somepublicgroup")
		}
		if _, ok := cfg.groupAllow["anothergroup"]; !ok {
			t.Fatalf("groupAllow missing anothergroup")
		}
	})

	t.Run("public mode discards groups section", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		content := strings.Join([]string{
			"bot:",
			"  token: test-token",
			"groups:",
			"  - id: invalid group id",
			"    topic: 4",
			"",
		}, "\n")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write config file: %v", err)
		}

		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load returned error: %v", err)
		}

		if !cfg.IsPublicMode() {
			t.Fatalf("IsPublicMode = false, want true")
		}
		if len(cfg.Groups) != 0 {
			t.Fatalf("Groups length = %d, want 0", len(cfg.Groups))
		}
		if len(cfg.groupTopics) != 0 {
			t.Fatalf("groupTopics length = %d, want 0", len(cfg.groupTopics))
		}
		if len(cfg.groupAllow) != 0 {
			t.Fatalf("groupAllow length = %d, want 0", len(cfg.groupAllow))
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
			name:    "invalid admin user id",
			content: "bot:\n  token: test\n  admin_user_ids: [0]\n",
			wantErr: "bot.admin_user_ids must contain positive integers",
		},
		{
			name: "invalid group id in private mode",
			content: strings.Join([]string{
				"bot:",
				"  token: test",
				"  admin_user_ids: [1001]",
				"groups:",
				"  - id: invalid group id",
				"    topic: 1",
				"",
			}, "\n"),
			wantErr: "groups[0].id is invalid",
		},
		{
			name: "missing groups in private mode",
			content: strings.Join([]string{
				"bot:",
				"  token: test",
				"  admin_user_ids: [1001]",
				"",
			}, "\n"),
			wantErr: "groups must contain at least one public group when bot.admin_user_ids is set",
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

			_, err := Load(path)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestNormalizePublicGroupID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{
			name:  "with at prefix",
			input: "@SomeGroup_123",
			want:  "somegroup_123",
		},
		{
			name:  "without at prefix",
			input: "SomeGroup_123",
			want:  "somegroup_123",
		},
		{
			name:    "empty value",
			input:   "",
			wantErr: "must not be empty",
		},
		{
			name:    "too short",
			input:   "@abc",
			wantErr: "must be a public group username",
		},
		{
			name:    "contains space",
			input:   "@bad group",
			wantErr: "must be a public group username",
		},
		{
			name:    "starts with number",
			input:   "@1groupname",
			wantErr: "must be a public group username",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizePublicGroupID(tt.input)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("NormalizePublicGroupID returned error: %v", err)
				}
				if got != tt.want {
					t.Fatalf("normalized id = %q, want %q", got, tt.want)
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
