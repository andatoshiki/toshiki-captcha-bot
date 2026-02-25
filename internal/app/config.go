package app

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

const defaultConfigPath = "config.yaml"

var publicGroupIDPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{4,31}$`)

type runtimeConfig struct {
	Bot         botConfig           `yaml:"bot"`
	Groups      []groupTopicConfig  `yaml:"groups"`
	groupAllow  map[string]struct{} `yaml:"-"`
	groupTopics map[string]int      `yaml:"-"`
	Captcha     captchaConfig       `yaml:"captcha"`
}

type botConfig struct {
	Token        string             `yaml:"token"`
	PollTimeout  time.Duration      `yaml:"poll_timeout"`
	AdminUserIDs []int64            `yaml:"admin_user_ids"`
	adminUsers   map[int64]struct{} `yaml:"-"`
}

type groupTopicConfig struct {
	ID    string `yaml:"id"`
	Topic int    `yaml:"topic"`
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
		},
		Groups: make([]groupTopicConfig, 0),
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

	adminUsers := make(map[int64]struct{}, len(c.Bot.AdminUserIDs))
	for _, userID := range c.Bot.AdminUserIDs {
		if userID <= 0 {
			return fmt.Errorf("bot.admin_user_ids must contain positive integers")
		}
		adminUsers[userID] = struct{}{}
	}
	c.Bot.adminUsers = adminUsers

	// Public mode is derived: when no admin_user_ids are configured,
	// group topic settings are intentionally ignored.
	if c.isPublicMode() {
		c.Groups = nil
		c.groupAllow = make(map[string]struct{})
		c.groupTopics = make(map[string]int)
	} else {
		groupAllow := make(map[string]struct{}, len(c.Groups))
		groupTopics := make(map[string]int, len(c.Groups))
		seen := make(map[string]struct{}, len(c.Groups))

		for i, group := range c.Groups {
			normalizedGroupID, err := normalizePublicGroupID(group.ID)
			if err != nil {
				return fmt.Errorf("groups[%d].id is invalid: %w", i, err)
			}
			c.Groups[i].ID = "@" + normalizedGroupID

			if _, exists := seen[normalizedGroupID]; exists {
				return fmt.Errorf("groups[%d].id duplicates @%s", i, normalizedGroupID)
			}
			seen[normalizedGroupID] = struct{}{}
			groupAllow[normalizedGroupID] = struct{}{}

			if group.Topic < 0 {
				return fmt.Errorf("groups[%d].topic must be greater than zero when set", i)
			}
			if group.Topic > 0 {
				groupTopics[normalizedGroupID] = group.Topic
			}
		}

		c.groupAllow = groupAllow
		c.groupTopics = groupTopics
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

func (c runtimeConfig) isPublicMode() bool {
	return len(c.Bot.adminUsers) == 0
}

func normalizePublicGroupID(raw string) (string, error) {
	id := strings.TrimSpace(raw)
	id = strings.TrimPrefix(id, "@")
	id = strings.ToLower(id)

	if id == "" {
		return "", fmt.Errorf("must not be empty")
	}

	if !publicGroupIDPattern.MatchString(id) {
		return "", fmt.Errorf("must be a public group username like @mygroup")
	}

	return id, nil
}

func normalizePublicGroupLookupID(raw string) string {
	id := strings.TrimSpace(raw)
	id = strings.TrimPrefix(id, "@")
	id = strings.ToLower(id)
	if id == "" {
		return ""
	}
	return id
}
