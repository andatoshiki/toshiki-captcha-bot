# toshiki-captcha-bot
> Telegram CAPTCHA bot for group join verification, written in Go with telebot v3.
## 1: Project overview
### 1.1: What this bot does
- Verifies new users with an emoji-image CAPTCHA before they can speak.
- Restricts joiners until they solve the challenge or timeout.
- Bans users who fail too many attempts or let the challenge expire.
- Supports optional per-group forum-topic delivery through `groups[].topic`.
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
  admin_user_ids: [123456789]

groups:
  - id: "@somepublicgroup"
    topic: 4

captcha:
  expiration: 1m
  cleanup_interval: 5s
  max_failures: 2
  failure_notice_ttl: 15s
```

### 3.2: Bot config reference
- `bot.token`: required Telegram bot token.
- `bot.poll_timeout`: long-poll timeout for update polling.
- `bot.admin_user_ids`: if empty, bot runs in public mode. if non-empty, bot runs in private mode and only configured admin IDs are treated as trusted operators.
- `groups`: optional group-topic map used only in private mode.
- `groups[].id`: public group username such as `@somepublicgroup`.
- `groups[].topic`: optional single forum topic id for that group.

### 3.3: Captcha config reference
- `captcha.expiration`: how long each challenge remains valid.
- `captcha.cleanup_interval`: janitor interval for expired challenge cleanup.
- `captcha.max_failures`: maximum wrong attempts before ban.
- `captcha.failure_notice_ttl`: how long failure notices stay before auto-delete.

### 3.4: Group topic behavior
- Only public groups are supported for topic routing.
- Private groups without a public `@username` are not supported and the bot will leave them.
- In public mode (`bot.admin_user_ids` empty), the bot discards `groups` config.
- In private mode, the bot resolves topic routing by matching incoming chat username to `groups[].id`.

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
- `/ping` is sender-restricted and only works for user IDs listed in `bot.admin_user_ids`.
- `/testcaptcha` trigger steps: (1) add your user ID to `bot.admin_user_ids`, (2) run `/testcaptcha` inside an allowed public group where the bot is already present, (3) bot issues a normal captcha challenge against the command sender for validation.
- `/testcaptcha` uses configured user ID checks only; Telegram chat-admin role is not required, but private chat dialogs and non-admin senders are ignored.
- Admin command suggestions are synced per configured admin user ID (private chat scope, and group member scope when groups are configured).
- If a non-admin sender runs `/ping` or `/testcaptcha`, the bot replies with an explicit access-denied message.
- Command scope sync state is stored in a hidden file beside your config path (example: `.config.yaml.command-scopes.json`) so removed admin IDs can be cleaned up on the next startup.

## 5: Development
### 5.1: Run tests
```bash
go test ./...
```

### 5.2: Useful local checks
```bash
go test ./... -run TestNormalizePublicGroupID

go run . -h
go run . -v
```

### 5.3: Key files
- `main.go`: startup, CLI handling, bot wiring, and runtime logging.
- `config.go`: YAML load, defaults, validation, mode derivation, and group-topic mapping.
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
- Confirm `groups[].id` values are valid public usernames when private mode is enabled.

### 7.2: Bot does not handle joins
- Verify bot is admin in the target group.
- Verify privacy mode and permissions allow required updates/actions.
- Confirm long polling is active and token is correct.
- If private mode is enabled, confirm at least one ID is set in `bot.admin_user_ids`.

### 7.3: Topic routing is not applied
- Ensure the chat username exists in `groups[].id` and a valid `groups[].topic` is set.
- Check startup logs for loaded `topic_mappings`.

## 8: License and attribution
### 8.1: License
- MIT License. See `LICENSE`.

### 8.2: Upstream and assets
- Built with `gopkg.in/telebot.v3`.
- CAPTCHA assets and gopher-themed image are included in this repository.
