# toshiki-captcha-bot
> Telegram CAPTCHA bot for group join verification, written in Go with telebot v3.
## 1: Project overview
### 1.1: What this bot does
- Verifies new users with an emoji-image CAPTCHA before they can speak.
- Restricts joiners until they solve the challenge or timeout.
- Bans users who fail too many attempts or let the challenge expire.
- Supports optional forum-topic delivery by parsing `bot.topic_link`.
- Runs with YAML config, CLI flags, and structured runtime logs.

### 1.2: Core behavior
- A join event triggers CAPTCHA generation and a challenge message.
- Correct answers progressively mark selected buttons.
- Passing users are unmuted and challenge messages are deleted.
- Failed or expired users are banned and a temporary notice is sent.

## 2: Quick start
### 2.1: Prerequisites
- Go installed (project currently targets Go modules with `go 1.17` in `go.mod`).
- A Telegram bot token from BotFather.
- Bot admin permissions in your target group.

### 2.2: Run locally
```bash
go mod tidy
go run . -c ./config.yaml
```

### 2.3: Build binary
```bash
go build -v -o toshiki-captcha-bot .
./toshiki-captcha-bot -c ./config.yaml
```

### 2.4: CLI flags
```text
-c, --config <path>   YAML configuration path (default: config.yaml)
-v, --version         Print version and exit
-h, --help            Show this help and exit
```

## 3: Configuration
### 3.1: Example config
```yaml
bot:
  token: "123456789:telegram-bot-token"
  poll_timeout: 10s
  public: true
  allowed_user_ids: []
  topic_link: "https://t.me/c/1234567890/4/77"

captcha:
  expiration: 1m
  cleanup_interval: 5s
  max_failures: 2
  failure_notice_ttl: 15s
```

### 3.2: Bot config reference
- `bot.token`: required Telegram bot token.
- `bot.poll_timeout`: long-poll timeout for update polling.
- `bot.public`: if `true`, any chat can use the bot. if `false`, only chats tied to configured allowed users are accepted.
- `bot.allowed_user_ids`: user IDs allowed to operate private mode. required when `bot.public` is `false`.
- `bot.topic_link`: optional Telegram topic URL reference. Leave empty to send messages to the chat root.

### 3.3: Captcha config reference
- `captcha.expiration`: how long each challenge remains valid.
- `captcha.cleanup_interval`: janitor interval for expired challenge cleanup.
- `captcha.max_failures`: maximum wrong attempts before ban.
- `captcha.failure_notice_ttl`: how long failure notices stay before auto-delete.

### 3.4: Topic link parsing behavior
- The bot extracts `message_thread_id` from `bot.topic_link` during config validation.
- Supported sources include:
  - Query parameter form: `?thread=<topic_id>`.
  - Path-based forms such as `/c/<chat>/<topic>/<message>`.
- If parsing fails, startup stops with a config validation error.

## 4: Captcha flow
### 4.1: Join to pass flow
1. User joins group.
2. Bot removes the raw join message.
3. Bot sends CAPTCHA image + inline emoji keyboard.
4. User selects matching emoji buttons in the same sequence as displayed in the image.
5. Bot unrestricts user after all required answers are solved.

### 4.2: Failure flow
1. Wrong answers increase failure count.
2. Reaching `captcha.max_failures` bans the user.
3. Bot posts a temporary failure notice and auto-removes it after `captcha.failure_notice_ttl`.

### 4.3: Expiration flow
1. Unsolved challenges expire after `captcha.expiration`.
2. Eviction handler bans the expired user.
3. Challenge and notice messages are cleaned up.

### 4.4: Utility command
- `/ping` replies with `pong` and measured latency in milliseconds.
- `/ping` is sender-restricted and only works for user IDs listed in `bot.allowed_user_ids`.

## 5: Development
### 5.1: Run tests
```bash
go test ./...
```

### 5.2: Useful local checks
```bash
go test ./... -run TestParseTopicIDReference

go run . -h
go run . -v
```

### 5.3: Key files
- `main.go`: startup, CLI handling, bot wiring, and runtime logging.
- `config.go`: YAML load, defaults, validation, and topic-link parsing.
- `handler.go`: join handling, challenge validation, failure/expiry paths.
- `helper.go`: caption generation, send options, and helpers.
- `config.example.yaml`: ready-to-copy config template.

## 6: Release and distribution
### 6.1: GitHub Actions workflow
- Workflow file: `.github/workflows/release.yml`.
- Non-tag pushes build the binary.
- `v*` tags run GoReleaser publish.

### 6.2: GoReleaser targets
- `linux`, `windows`, `darwin` on `amd64` and `arm64`.
- Additional `linux/armv7` build via dedicated matrix entry.
- Release archives include `README.md`, `LICENSE`, and `config.example.yaml`.

### 6.3: Version metadata
- `-v` reads build metadata from linker-injected vars:
  - `main.Version`
  - `main.Commit`
  - `main.BuildTime`

## 7: Operations and troubleshooting
### 7.1: Startup fails on config
- Confirm `bot.token` is non-empty.
- Confirm all duration values are greater than zero.
- Confirm `bot.topic_link` is either empty or a valid Telegram link.

### 7.2: Bot does not handle joins
- Verify bot is admin in the target group.
- Verify privacy mode and permissions allow required updates/actions.
- Confirm long polling is active and token is correct.
- If `bot.public` is `false`, confirm at least one chat admin user ID is listed in `bot.allowed_user_ids`.

### 7.3: Topic routing is not applied
- Ensure `bot.topic_link` points to the intended forum topic.
- Check startup logs for resolved `topic_thread_id`.

## 8: License and attribution
### 8.1: License
- MIT License. See `LICENSE`.

### 8.2: Upstream and assets
- Built with `gopkg.in/telebot.v3`.
- CAPTCHA assets and gopher-themed image are included in this repository.
