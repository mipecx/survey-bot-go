# Survey Bot Go

A high-performance, concurrent Telegram bot engine for structured surveys and automated data collection. Built with Go and PostgreSQL.

## Features
- **Concurrent Processing**: Leverages Go routines to handle multiple users simultaneously.
- **Dynamic Survey Logic**: Flexible JSONB-based storage for survey answers, allowing easy form updates without database migrations.
- **Layered Architecture**: Clear separation between Transport (Telegram), Service (Logic), and Repository (Database) layers.
- **Production-ready Logging**: Structured logging with `slog`.

## Tech Stack
- **Language**: Go (Golang)
1.25+
- **Database**: PostgreSQL (pgxpool for connection pooling)
- **API**: telegram-bot-api/v5
- **Environment**: Cleanenv / Godotenv

## Getting Started

### Prerequisites
- Go 1.25 or higher
- PostgreSQL instance

### Installation
1. Clone the repository:
```bash
git clone [https://github.com/mipecx/survey-bot-go.git](https://github.com/mipecx/survey-bot-go.git)
cd survey-bot-go
```

2. Set up your environment variables in a `.env` file:

    Code snippet

    ```
    BOT_TOKEN=your_telegram_bot_token
    DATABASE_URL=postgres://user:password@localhost:5432/dbname
    ```

3. Run the application:

    Bash

    ```
    go run cmd/main.go
    ```


## Project Structure

- `cmd/`: Application entry point.

- `internal/handler/`: Telegram update handlers.

- `internal/service/`: Business logic and survey flow management.

- `internal/repository/`: Database interactions.

- `internal/models/`: Core data structures.
