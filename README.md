# Site Monitor

A website monitoring service that periodically checks the availability of websites and sends Telegram notifications when a site goes down or comes back up.

## Features

- Periodic HTTP checks for monitored websites
- Telegram notifications on status changes
- REST API for managing monitors
- Telegram bot for managing monitors and viewing status
- Check history with response times

## Tech Stack

- **Go** — main language
- **PostgreSQL** — database
- **chi** — HTTP router
- **sqlx** — database access
- **telebot v3** — Telegram bot
- **Docker** — containerization

## Getting Started

### Running with Docker (recommended)

1. Clone the repository:
```bash
git clone https://github.com/peshk1n/site-monitor.git
cd site-monitor
```

2. Create a `.env` file based on `.env.example`:
```bash
cp .env.example .env
```

3. Fill in the environment variables in `.env` (see [Environment Variables](#environment-variables))

4. Start the application:
```bash
docker-compose up --build
```

Migrations are applied automatically on first run.

### Running without Docker

#### Prerequisites

- Go 1.22+
- PostgreSQL

#### Installation

1. Clone the repository:
```bash
git clone https://github.com/peshk1n/site-monitor.git
cd site-monitor
```

2. Install dependencies:
```bash
go mod download
```

3. Create a `.env` file based on `.env.example`:
```bash
cp .env.example .env
```

4. Fill in the environment variables in `.env` (see [Environment Variables](#environment-variables))

5. Apply migrations:
```bash
psql -U postgres -d sitemonitor -f migrations/up/001_create_monitors.up.sql
psql -U postgres -d sitemonitor -f migrations/up/002_create_checks.up.sql
```

6. Run the application:
```bash
go run cmd/server/main.go
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | PostgreSQL connection string (local) |
| `DATABASE_URL_DOCKER` | PostgreSQL connection string (Docker) |
| `SERVER_PORT` | HTTP server port |
| `TELEGRAM_TOKEN` | Telegram bot token from @BotFather |
| `TELEGRAM_CHAT_ID` | Your Telegram chat ID |
| `POSTGRES_DB` | PostgreSQL database name |
| `POSTGRES_USER` | PostgreSQL username |
| `POSTGRES_PASSWORD` | PostgreSQL password |

## REST API

### Monitors

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/monitors` | Get all monitors |
| `POST` | `/api/v1/monitors` | Create a new monitor |
| `GET` | `/api/v1/monitors/{id}` | Get monitor by ID |
| `DELETE` | `/api/v1/monitors/{id}` | Delete a monitor |

### Checks

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/monitors/{id}/checks` | Get check history for a monitor |
| `GET` | `/api/v1/monitors/{id}/checks/last` | Get last check for a monitor |

### Example

Create a monitor:
```bash
curl -X POST http://localhost:8080/api/v1/monitors \
  -H "Content-Type: application/json" \
  -d '{"url": "https://google.com", "interval": 60, "timeout": 10}'
```

## Telegram Bot Commands

| Command | Description |
|---------|-------------|
| `/start` | Show available commands |
| `/list` | List all monitors with current status |
| `/add` | Add a new website to monitor |
| `/delete` | Remove a monitor |
| `/status` | View last check details for a monitor |
