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

## Getting Started

### Prerequisites

- Go 1.22+
- PostgreSQL

### Installation

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
psql -U postgres -d sitemonitor -f migrations/001_create_monitors.sql
psql -U postgres -d sitemonitor -f migrations/002_create_checks.sql
```

6. Run the application:
```bash
go run cmd/server/main.go
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | PostgreSQL connection string |
| `SERVER_PORT` | HTTP server port |
| `TELEGRAM_TOKEN` | Telegram bot token from @BotFather |
| `TELEGRAM_CHAT_ID` | Your Telegram chat ID |

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
  -d '{"url": "https://google.com", "interval": 60}'
```

## Telegram Bot Commands

| Command | Description |
|---------|-------------|
| `/start` | Show available commands |
| `/list` | List all monitors with current status |
| `/add` | Add a new website to monitor |
| `/delete` | Remove a monitor |
| `/status` | View last check details for a monitor |
