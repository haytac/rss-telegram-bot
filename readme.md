# Go RSS to Telegram Bot

[![Go Version](https://img.shields.io/badge/go-1.24+-blue.svg)](https://golang.org/dl/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
<!-- Optional: Add build status, code coverage, etc. if you set up CI/CD -->
<!-- [![Build Status](https://travis-ci.org/yourusername/rss-telegram-bot.svg?branch=main)](https://travis-ci.org/yourusername/rss-telegram-bot) -->

A robust and extensible Go application that monitors RSS feeds at user-defined frequencies and sends richly formatted updates to configured Telegram bots.

## ✨ Features

*   **RSS Feed Monitoring:**
    *   Fetches multiple RSS feeds concurrently using `gofeed`.
    *   Detects new entries since the last fetch (prevents duplicates).
    *   Supports HTTP caching (`If-Modified-Since`, `ETag`) for efficient fetching.
    *   Individual feed scheduling (e.g., every 5 minutes, hourly).
*   **Telegram Integration:**
    *   Sends new feed items to configured Telegram bots using the Telegram Bot API (`go-telegram-bot-api/v5`).
    *   Supports multiple target chats/channels per feed or globally.
*   **Content Formatting & Delivery:**
    *   **Rich Text:** Preserves rich-text formatting (bold, italic, links) using Telegram's `ParseModeHTML`.
    *   **Media Handling:** (Planned/Partially Implemented)
        *   Supports images, videos, audio, documents from post content and enclosures.
        *   (Planned) Handles large images/media appropriately (e.g., sending as files).
        *   (Planned) Configurable media filters (regex/CSS selectors).
    *   **Emoji Support:** Automatically replaces emoji shortcodes (e.g., `:smile:`) with Unicode emojis using `kyokomi/emoji/v2`.
    *   **Title & Author Control:** Configurable omission of generic feed titles; includes author names when available.
    *   **Message Splitting:** Automatically splits messages exceeding Telegram's character limit, preserving formatting.
    *   **Telegraph Integration:** (Planned) Optionally send long content as Telegraph posts.
    *   **Customizable Templates:** Uses Go's `text/template` for user-defined message and title formats per feed.
    *   **Hashtags:** Supports adding configurable hashtags.
*   **Persistence & Configuration:**
    *   **SQLite Database:** Stores RSS feed configurations, user settings, formatting preferences, and processed item history.
    *   **Database Migrations:** Uses `golang-migrate` for schema management.
    *   **Configuration File:** Supports YAML configuration (`config.yml`) for global settings, database paths, logging, etc.
    *   **Environment Variables:** Configuration can be overridden by environment variables (e.g., `RSS_BOT_ENCRYPTION_KEY`).
*   **Extensibility & Maintainability:**
    *   Modular design with separation of concerns (database, RSS, Telegram, CLI, formatting).
    *   Uses interfaces and dependency injection for extensibility.
    *   Comprehensive structured logging with `zerolog` (console and file output, different levels).
*   **Operational Features:**
    *   **Proxy Support:** Configurable HTTP/SOCKS5 proxies per feed for RSS fetching and globally for Telegram API requests. Includes proxy validation.
    *   **OPML Support:** (Planned) Import and export feed lists.
    *   **Rate Limiting:** Respects Telegram API rate limits using `golang.org/x/time/rate`.
    *   **Error Recovery:** Includes retry mechanisms with exponential backoff for RSS fetches.
    *   **Graceful Shutdown:** Handles SIGINT/SIGTERM for clean shutdown.
    *   **Dockerization:** Includes `Dockerfile` and `docker-compose.yml` for easy deployment.
    *   **Monitoring:** Exposes Prometheus metrics (e.g., feeds processed, errors) via an HTTP endpoint.
*   **User-Friendly CLI:**
    *   Built with `cobra`.
    *   CRUD operations for feeds, proxies, bot tokens, formatting profiles.
    *   Database backup and restore commands.
    *   `--dry-run` mode for testing.
    *   Verbose output for debugging.

## 🛠️ Prerequisites

*   Go (version 1.24 or higher recommended for building locally)
*   Docker & Docker Compose (for running the application in a container)
*   `golang-migrate/migrate` CLI (for manual database migrations)
    *   Install with SQLite support: `go install -tags 'sqlite3' github.com/golang-migrate/migrate/v4/cmd/migrate@latest`
*   A Telegram Bot Token (get one from @BotFather on Telegram)
*   A Telegram Chat ID (for a private chat, group, or channel where the bot will send messages)

## 🚀 Getting Started

### 1. Clone the Repository

```bash
git clone https://github.com/[YOUR_USERNAME]/rss-telegram-bot.git
cd rss-telegram-bot
```

### 2. Configuration

Copy the example configuration file and customize it:

```bash
cp config.yml.example config.yml
nano config.yml
```

**Key settings to review in `config.yml`:**

*   `database_path`: Path to the SQLite database file *inside the Docker container* (default: `/app/data/rss_bot.db`).
*   `log`: Logging level, console/file output.
*   `metrics_port`: Port for Prometheus metrics.
*   `encryption_key`: **CRITICAL for security.** Set a long, random string. For demo purposes, the application will use an insecure default if this is empty, but will warn you.
    *   You can also set this via the `RSS_BOT_ENCRYPTION_KEY` environment variable.

### 3. Build the Docker Image

```bash
docker compose build
```

### 4. Prepare Data Directory and Database

*   Create the data directory on your host (this will be mounted into the container):
    ```bash
    mkdir -p ./data
    sudo chmod -R 777 ./data # For local development to avoid permission issues
    ```
*   Run database migrations (recommended before first run):
    ```bash
    # Ensure ./data/rss_bot.db is removed if you want a fresh start or fixed a "dirty" state
    # sudo rm -f ./data/rss_bot.db
    migrate -database "sqlite3://./data/rss_bot.db" -path ./internal/database/migrations up
    ```

### 5. Initial Setup via CLI

These commands are run using `docker compose run`, which executes them in a temporary container interacting with your persistent database.

*   **Add your Telegram Bot Token:**
    Replace `YOUR_ACTUAL_TELEGRAM_BOT_TOKEN`.
    ```bash
    docker compose run --rm rss-bot bot add YOUR_ACTUAL_TELEGRAM_BOT_TOKEN --description "My RSS Bot"
    ```
    Note the `ID` returned (e.g., `1`).

*   **Add an RSS Feed:**
    Replace `BOT_ID` (from the previous step), and `YOUR_TELEGRAM_CHAT_ID`.
    ```bash
    docker compose run --rm rss-bot feed add https://hnrss.org/frontpage \
      --title "Hacker News" \
      --bot-token-id 1 \
      --chat-id "YOUR_TELEGRAM_CHAT_ID" \
      --freq 300 # Check every 5 minutes
    ```

### 6. Run the Application

To run the main service and see logs in your terminal:
```bash
docker compose up rss-bot
```
To run in detached (background) mode:
```bash
docker compose up -d rss-bot
```
View logs when detached:
```bash
docker compose logs -f rss-bot
```

### 7. Stopping the Application
```bash
docker compose down
```
(Your data in `./data` will persist).

## ⚙️ CLI Usage

The application provides a CLI for management. All commands are run via `docker compose run --rm rss-bot ...` or by executing the binary directly if built locally.

```bash
# General help
docker compose run --rm rss-bot --help

# Feed management
docker compose run --rm rss-bot feed --help
docker compose run --rm rss-bot feed add <url> --bot-token-id <id> --chat-id <chat_id> [flags]
docker compose run --rm rss-bot feed list
# docker compose run --rm rss-bot feed update <feed_id> [flags] # (Planned)
# docker compose run --rm rss-bot feed remove <feed_id>       # (Planned)

# Bot token management
docker compose run --rm rss-bot bot --help
docker compose run --rm rss-bot bot add <raw_bot_token> [flags]
docker compose run --rm rss-bot bot list

# Proxy management
docker compose run --rm rss-bot proxy --help
docker compose run --rm rss-bot proxy add <name> <type> <address> [flags] # type: http, https, socks5
docker compose run --rm rss-bot proxy list
docker compose run --rm rss-bot proxy validate <proxy_id>

# Formatting profile management
docker compose run --rm rss-bot formatprofile --help
docker compose run --rm rss-bot formatprofile add <profile_name> -c <config_file.json> [flags]
docker compose run --rm rss-bot formatprofile list

# Database management
docker compose run --rm rss-bot db --help
docker compose run --rm rss-bot db backup [-o /app/data/backup_name.db]
docker compose run --rm rss-bot db restore /app/data/backup_name.db

# Run the main service (usually done via `docker compose up`)
# docker compose run --rm rss-bot run
```

**Global Flags:**
*   `--config <path>`: Specify a config file path.
*   `--dry-run`: Simulate actions without making changes or sending messages.

## 🔧 Building Locally (Optional)

If you have Go installed (version 1.24+):

1.  Install dependencies:
    ```bash
    go mod tidy
    ```
2.  Build the binary:
    ```bash
    go build -o rss-telegram-bot ./cmd/rss-telegram-bot/main.go
    ```
3.  Run with local config:
    ```bash
    ./rss-telegram-bot --config ./config.yml run
    ```
    (Ensure `database_path` in `config.yml` points to a local path like `./data/rss_bot.db` for local runs).

## 📈 Monitoring

Prometheus metrics are exposed on the port defined by `metrics_port` in `config.yml` (default `/metrics` path). Example: `http://localhost:9090/metrics` if `metrics_port: ":9090"` and port 9090 is mapped from the container.

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request or open an Issue.
(Optional: Add more specific contribution guidelines, code of conduct, etc.)

## 📜 License

This project is licensed under the MIT License - see the [LICENSE.md](LICENSE.md) file for details (you'll need to create this file).

---

This provides a good starting point. You'll want to:
*   Fill in `[YOUR_USERNAME]`.
*   Create a `LICENSE.md` file (e.g., with MIT License text).
*   Update the "Planned" features as you implement them.
*   Add more detailed explanations for complex configuration options (like message templates) as they are developed.
*   If you set up CI/CD, add badges for build status, etc.
```

**To create a `LICENSE.md` file with the MIT License:**

Create a file named `LICENSE.md` in your project root with the following content, replacing `[year]` and `[fullname]` :

```
MIT License

Copyright (c) [year] [fullname]

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```