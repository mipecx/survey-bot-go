# survey-bot-go

A production-ready Telegram bot for structured client surveys and automated lead collection. Built for the **Архитектура любви** matchmaking project.

Handles multi-step survey flows, lead notifications, Google Sheets integration, and per-request structured logging - all in a clean layered architecture.

## Features

- **Concurrent user handling** - per-user mutex ensures sequential processing without blocking other users
- **Multi-step survey engine** - branching forms, inline keyboards, validation, and auto-advance
- **Contact profile** - phone, birth date, city, and gender collected once and reused across all forms
- **Google Sheets integration** - every completed survey is appended to a dedicated worksheet automatically
- **Admin notifications** - formatted HTML summaries sent to all configured admins via Telegram
- **Per-request context logging** - every log line carries `request_id` and `user_id` for full traceability
- **Database migrations** - managed with `golang-migrate`, applied automatically at startup
- **Graceful shutdown** - in-flight updates are drained before the process exits

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.26 |
| Bot API | go-telegram-bot-api/v5 |
| Database | PostgreSQL 15 (pgx/v5 pool) |
| Migrations | golang-migrate |
| Sheets | Google Sheets API v4 |
| Logging | log/slog (JSON) |
| Config | godotenv |
| Container | Docker + Docker Compose |

## Project Structure

```
survey-bot-go/
├── cmd/bot/          # Application entry point
├── internal/
│   ├── app/          # Wiring: initialises all components and runs the update loop
│   ├── bot/          # Transport layer: handlers, keyboards, admin notifier
│   ├── config/       # Environment-based configuration
│   ├── ctxlog/       # Per-request logger stored in context.Context
│   ├── models/       # Domain types (User)
│   ├── repository/   # UserRepository interface
│   │   └── postgres/ # PostgreSQL implementation (pgxpool)
│   ├── service/      # Business logic: survey flow, validation, notifications
│   └── sheets/       # Google Sheets client
└── migrations/       # SQL migration files (golang-migrate)
```

## Getting Started

### Prerequisites

- Go 1.26+
- Docker and Docker Compose
- A Telegram bot token (`@BotFather`)
- A Google Cloud Service Account with Sheets API enabled

### Configuration

Copy the example env file and fill in your values:

```bash
cp .env.example .env
```

| Variable | Required | Description |
|---|---|---|
| `BOT_TOKEN` | ✅ | Telegram bot token |
| `DATABASE_URL` | ✅ | PostgreSQL connection string |
| `ADMIN_IDS` | ✅ | Comma-separated Telegram user IDs for notifications |
| `GOOGLE_SHEETS_ID` | ✅ | Google Spreadsheet ID from the URL |
| `GOOGLE_CREDENTIALS_FILE` | ✅ | Path to Service Account JSON key file |
| `COMMUNITY_URL` | - | URL for community/gift buttons (default: `https://t.me/default_group`) |
| `GIFT_FILE_ID` | - | Telegram `file_id` of the gift PDF |
| `WELCOME_IMAGE_ID` | - | Telegram `file_id` of the welcome photo |
| `LOG_LEVEL` | - | `debug`, `info`, `warn`, `error` (default: `info`) |

### Running with Docker

```bash
docker compose up -d --build
```

This starts three services in order:
1. `db` - PostgreSQL, waits until healthy
2. `migrate` - applies pending migrations, then exits
3. `bot` - starts only after migrations complete

### Running locally

```bash
# Start the database
docker compose up -d db

# Apply migrations
migrate -path ./migrations -database "postgres://user:password@localhost:5432/arch_bot?sslmode=disable" up

# Run the bot
go run cmd/bot/main.go
```

## Database Migrations

Migrations live in `migrations/` and follow the `golang-migrate` naming convention:

```
000001_create_users.up.sql
000001_create_users.down.sql
000002_add_pending_form.up.sql
...
```

To create a new migration:

```bash
migrate create -ext sql -dir migrations -seq <name>
```

To roll back one step:

```bash
migrate -path ./migrations -database "..." down 1
```

## Architecture Notes

**Survey flow**

```
/start
  └── new_user form (name only)
      └── main menu
          └── any feature button
              ├── contact form (phone, birth date, city, gender) - first time only
              │   └── target form
              └── target form (if contact already collected)
```

**Message editing strategy**

The bot edits its previous message in place for most transitions (no chat clutter). It falls back to sending a new message when:
- The previous message used a reply keyboard (`InputPhone`)
- Telegram rejects the edit (message too old, etc.)
- The form ends after a free-text question

**Callback data encoding**

To stay within Telegram's 64-byte `callback_data` limit, survey option buttons encode the button index rather than the full text: `stepID:index`. The index is resolved back to option text in `ProcessCallback` via `resolveOption`.
