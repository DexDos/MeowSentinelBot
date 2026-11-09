# 🐱 MeowSentinelBot `v1.0.0 (mlp)`

> **Your personal and collaborative media tracker for Telegram.**  
> Built with Go, [`telego`](https://github.com/mymmrac/telego), and pure-Go SQLite (`modernc.org/sqlite`), engineered for containerized cloud deployment on Google Cloud Run.

---

## 🌟 Usage Features

MeowSentinel is a Minimum Lovable Product (MLP) designed for seamless personal tracking and collaborative watchlists with friends or family.

### 📚 Stack & Media Management
- **Personal & Shared Stacks:** Organize media into independent lists ("Weekend Watchlist", "Anime 2026", "Classic Cinema").
- **Local 1-Based Indexing:** Records are displayed with clean, sequential numbers (`#1`, `#2`, `#3...`) rather than internal database primary keys.
- **Dynamic Re-indexing on Deletion:** Deleting an item from the middle of a stack automatically and cleanly re-numbers remaining items sequentially (`1..N`) with zero gaps.
- **Dual Selection Experience:**
  - **Inline Keyboards:** Quick-pick button grid (`[ #1 ] [ #2 ] [ #3 ]`) maintaining underlying database primary keys in callback payloads.
  - **Natural Chat Input:** Type `#1`, `1`, or `2` directly in chat when viewing a stack to inspect that entry.
- **Flexible Media Types:** Built-in categorizers for Movies, TV Series, Anime, Cartoons, or custom user-defined types.

### 🎬 Watching Progress & Collaboration
- **Quick Progress Steppers:** Tap `◀️ -1 Ep`, `▶️ +1 Ep`, `⏪ -1 S`, or `⏩ +1 S` for instant inline watching progress updates.
- **Smart Progress Parser:** Enter progress in freeform text (e.g. `2 5`, `s2 e5`, `S02E05`, or `season 3 episode 4`).
- **Collaborative 6-Star Rating System:**
  - ⭐️ Nasty (1/6)
  - ⭐️⭐️ Bad (2/6)
  - ⭐️⭐️⭐️ Ick (3/6)
  - ⭐️⭐️⭐️⭐️ Worth Meowing (4/6)
  - ⭐️⭐️⭐️⭐️⭐️ Nice One (5/6)
  - 🌟✨🌟✨🌟 **MEOW (6/6)**
  - Calculates dynamic average community ratings while tracking each member's individual vote.
- **Temporary Invite Codes & Deep Links:** Stack owners can generate expiring invite codes (`MEOW-XXXX`) or one-click Telegram deep links (`https://t.me/<bot>?start=CODE`) to invite friends with role separation (`Owner` vs `Shared Member`).

---

## 🛠 Technical Details: Growth & Forks Potential

MeowSentinel is built around clean architecture and Go standard library idioms, making it easy to extend, fork, or adapt to enterprise relational databases.

```
                    ┌───────────────────────────┐
                    │  Telegram Bot API Webhook │
                    └─────────────┬─────────────┘
                                  │ HTTPS (X-Telegram-Bot-Api-Secret-Token)
                    ┌─────────────▼─────────────┐
                    │ net/http Server (:8080)   │
                    │   - GET  /healthz         │
                    │   - POST /bot             │
                    └─────────────┬─────────────┘
                                  │
                    ┌─────────────▼─────────────┐
                    │  telego Bot & Handlers    │
                    │  (State-driven FSM Store) │
                    └─────────────┬─────────────┘
                                  │
                    ┌─────────────▼─────────────┐
                    │  Domain Application Svc   │
                    │  (Auth, Permissions, Inv) │
                    └─────────────┬─────────────┘
                                  │
                    ┌─────────────▼─────────────┐
                    │  storage.Storage Interf.  │
                    └─────────────┬─────────────┘
                                  │
        ┌─────────────────────────┴─────────────────────────┐
        ▼                                                   ▼
┌───────────────────────────────┐           ┌───────────────────────────────┐
│ modernc.org/sqlite (Default)  │           │ Potential Fork: PostgreSQL    │
│ Pure Go, WAL mode, CGO-free   │           │ pgx / Cloud SQL / MySQL       │
└───────────────────────────────┘           └───────────────────────────────┘
```

### 1. Interface-Driven Storage Layer (`storage.Storage`)
The database interaction is decoupled behind the `storage.Storage` interface in `internal/storage/storage.go`.
- **Current Driver:** `modernc.org/sqlite` — compiled statically with `CGO_ENABLED=0`, requiring no external C libraries or GCC dependencies.
- **Forking to External Databases:** Implementing the `storage.Storage` contract allows drop-in support for **PostgreSQL** (`pgx`), **MySQL**, or distributed databases without altering business logic or bot handlers.
- **SQLite Cloud Storage:** On Google Cloud Run or Kubernetes, SQLite persistence can be backed by Cloud Storage FUSE mounts (`DB_PATH=/data/bot.db`) or Litestream replication.

### 2. High-Throughput Webhook Engine
- Migrated from long polling to `bot.UpdatesViaWebhook` using `telego.WebhookHTTPServeMux`.
- Standard `net/http.Server` with production timeouts (`ReadHeaderTimeout: 5s`, `ReadTimeout: 10s`, `WriteTimeout: 15s`, `IdleTimeout: 60s`).
- Health check endpoint `GET /healthz` for Kubernetes or Cloud Run liveness/readiness probes.
- Multi-step graceful shutdown on `SIGINT` / `SIGTERM` ensuring in-flight webhooks complete and database WAL commits finalize cleanly.

### 3. Lightweight, Hardened Container
- Multi-stage `Dockerfile` creating a static binary under an unprivileged user (`botuser:botgroup`, UID 10001).
- Includes `ca-certificates` for TLS verification and `tzdata` for timezone-aware logging.
- Minimal image footprint (~25MB total).

### 4. Roadmap & Expansion Opportunities
- 🔍 **Metadata Providers:** Hook into TMDB, AniList, or IMDb APIs to auto-complete posters, synopses, and genres upon title entry.
- 🔔 **Episode Release Notifications:** Background workers querying RSS / API feeds to notify stack members when new episodes drop.
- 🌐 **Internationalization (i18n):** Presentation layer templates are centralized in `internal/bot/presenters/`, ready for multi-language dictionary mapping.

---

## 🔒 Security Overview

MeowSentinel is built following Application Security (AppSec) best practices and OWASP principles.

### 1. SQL Injection (SQLi) Prevention
- **Strict Parameterization:** 100% of SQL queries in `internal/storage/sqlite/sqlite.go` use positional parameterized queries (`?`).
- **No Query Concatenation:** String interpolation (`fmt.Sprintf`) is prohibited for query construction.
- Tested and verified against common vectors (`' OR 1=1 --`, `'; DROP TABLE stacks; --`, `UNION SELECT`).

### 2. Telegram Markup Injection (XSS / Entity Spoofing)
- **HTML Sanitization:** All user-controlled fields (`stack.Title`, `entry.Title`, media types, and interactive prompts) are processed through `html.EscapeString(...)` prior to message assembly.
- Malformed inputs (e.g., `<script>`, unclosed `<b>`, broken XML entities) cannot break Telegram entity parsing or disrupt message delivery.

### 3. Insecure Direct Object Reference (IDOR) Mitigation
- **Contextual Access Control:** Every handler extracts `userID` directly from Telegram's cryptographically signed update context (`msg.From.ID` / `query.From.ID`).
- **Strict Boundary Checks:** The service layer asserts that `userID` is an authorized member (`IsStackMember`) or owner (`RoleOwner`) before querying, updating, stepping, rating, or deleting any stack or entry.
- An unauthorized user cannot manipulate another user's records by guessing database IDs or spoofing callback data.

### 4. Webhook Authentication
- Inbound webhook POST requests are validated against the `X-Telegram-Bot-Api-Secret-Token` header using the `WEBHOOK_SECRET` environment variable.
- Non-matching or unauthenticated HTTP requests are rejected immediately with `401 Unauthorized`.

### 5. Memory Safety & Concurrency
- **Zero Data Races:** In-memory conversational state (`fsm.Store`) uses `sync.RWMutex` with defensive deep copies on session data, verified clean under `go test -race ./...`.
- **TTL Eviction:** Stale conversational sessions are garbage-collected by a background cleaner ticker to prevent memory bloat.
- **Resource Cleanup:** All database row sets (`sql.Rows`) are closed via `defer rows.Close()` and validated with `rows.Err()`.
- **DoS Protection:** Enforces a 128-character maximum length on titles and 64-character limit on media types, protecting against oversized payload floods.

---

## 🚀 Quick Start

### Local Development
```bash
# 1. Clone repository
git clone https://github.com/your-username/MeowSentinelBot.git
cd MeowSentinelBot

# 2. Configure environment
export TELEGRAM_BOT_TOKEN="your-bot-token"
export WEBHOOK_SECRET="your-high-entropy-secret"

# 3. Run automated tests (with race detection)
go test -race -v ./...

# 4. Start local bot
go run ./cmd/main.go
```

### Docker Build & Run
```bash
# Build image
docker build -t meow-sentinel-bot:v1.0.0 .

# Run container
docker run -d \
  -p 8080:8080 \
  -e TELEGRAM_BOT_TOKEN="your-bot-token" \
  -e WEBHOOK_SECRET="your-webhook-secret" \
  -v $(pwd)/data:/data \
  --name meow-bot \
  meow-sentinel-bot:v1.0.0
```

---

## 📄 License
This project is open-source under the [MIT License](LICENSE).
