package settings

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

const DefaultConfigPath = "config.yaml"

var publicGroupIDPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{4,31}$`)

type RuntimeConfig struct {
	Bot         BotConfig           `yaml:"bot"`
	Groups      []GroupTopicConfig  `yaml:"groups"`
	groupAllow  map[string]struct{} `yaml:"-"`
	groupTopics map[string]int      `yaml:"-"`
	Captcha     CaptchaConfig       `yaml:"captcha"`
}

type BotConfig struct {
	Token          string             `yaml:"token"`
	PollTimeout    time.Duration      `yaml:"poll_timeout"`
	RequestTimeout time.Duration      `yaml:"request_timeout"`
	AdminUserIDs   []int64            `yaml:"admin_user_ids"`
	adminUsers     map[int64]struct{} `yaml:"-"`
}

type GroupTopicConfig struct {
	ID    string `yaml:"id"`
	Topic int    `yaml:"topic"`
}

type CaptchaConfig struct {
	Expiration       time.Duration `yaml:"expiration"`
	CleanupInterval  time.Duration `yaml:"cleanup_interval"`
	MaxFailures      int           `yaml:"max_failures"`
	FailureNoticeTTL time.Duration `yaml:"failure_notice_ttl"`
}

func DefaultRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		Bot: BotConfig{
			PollTimeout:    10 * time.Second,
			RequestTimeout: 30 * time.Second,
		},
		Groups: make([]GroupTopicConfig, 0),
		Captcha: CaptchaConfig{
			Expiration:       1 * time.Minute,
			CleanupInterval:  5 * time.Second,
			MaxFailures:      2,
			FailureNoticeTTL: 15 * time.Second,
		},
	}
}

func Load(path string) (RuntimeConfig, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultConfigPath
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("read config file %q: %w", path, err)
	}

	cfg := DefaultRuntimeConfig()
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return RuntimeConfig{}, fmt.Errorf("decode YAML config file %q: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return RuntimeConfig{}, fmt.Errorf("invalid config file %q: %w", path, err)
	}

	return cfg, nil
}

func (c *RuntimeConfig) Validate() error {
	if strings.TrimSpace(c.Bot.Token) == "" {
		return fmt.Errorf("bot.token is required")
	}
	if c.Bot.PollTimeout <= 0 {
		return fmt.Errorf("bot.poll_timeout must be greater than zero")
	}
	if c.Bot.RequestTimeout <= 0 {
		return fmt.Errorf("bot.request_timeout must be greater than zero")
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
	if c.IsPublicMode() {
		c.Groups = nil
		c.groupAllow = make(map[string]struct{})
		c.groupTopics = make(map[string]int)
	} else {
		if len(c.Groups) == 0 {
			return fmt.Errorf("groups must contain at least one public group when bot.admin_user_ids is set")
		}

		groupAllow := make(map[string]struct{}, len(c.Groups))
		groupTopics := make(map[string]int, len(c.Groups))
		seen := make(map[string]struct{}, len(c.Groups))

		for i, group := range c.Groups {
			normalizedGroupID, err := NormalizePublicGroupID(group.ID)
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

func (c RuntimeConfig) IsPublicMode() bool {
	return len(c.Bot.adminUsers) == 0
}

func NormalizePublicGroupID(raw string) (string, error) {
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

func NormalizePublicGroupLookupID(raw string) string {
	id := strings.TrimSpace(raw)
	id = strings.TrimPrefix(id, "@")
	id = strings.ToLower(id)
	if id == "" {
		return ""
	}
	return id
}

func (c RuntimeConfig) HasAdminUser(userID int64) bool {
	if userID <= 0 {
		return false
	}
	_, ok := c.Bot.adminUsers[userID]
	return ok
}

func (c RuntimeConfig) AdminUserCount() int {
	return len(c.Bot.adminUsers)
}

func (c RuntimeConfig) GroupCount() int {
	return len(c.Groups)
}

func (c RuntimeConfig) TopicMappingCount() int {
	return len(c.groupTopics)
}

func (c RuntimeConfig) GroupsList() []GroupTopicConfig {
	out := make([]GroupTopicConfig, len(c.Groups))
	copy(out, c.Groups)
	return out
}

func (c RuntimeConfig) TopicForChatUsername(username string) int {
	if c.IsPublicMode() {
		return 0
	}
	groupID := NormalizePublicGroupLookupID(username)
	if groupID == "" {
		return 0
	}
	return c.groupTopics[groupID]
}

func (c RuntimeConfig) IsAllowedPublicGroupUsername(username string) bool {
	if c.IsPublicMode() {
		return true
	}
	groupID := NormalizePublicGroupLookupID(username)
	if groupID == "" {
		return false
	}
	_, ok := c.groupAllow[groupID]
	return ok
}
