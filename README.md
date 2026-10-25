# Askord

A ticket bot for Discord with a web dashboard to configure it.

## Features

- **Ticket panel** in Discord with one button per ticket type.
- **Configurable ticket types**: name, emoji, category, staff role and your own form with questions. Reorder them by dragging.
- **Transcripts** of every ticket when it is closed, with export. The content is purged automatically 90 days after closing, and it can also be deleted by hand.
- **Optional AI assistant**: each server connects its own LLM (any provider compatible with the OpenAI API) and sets general guidelines and per-ticket-type instructions. The assistant answers in tickets and notifies the staff when it cannot solve the problem.
- **Web dashboard** with Discord login, statistics and per-server settings.

## How it is built

A monorepo with two parts:

- `backend/`: Go. A single process runs the Discord bot and the HTTP API. It uses Postgres, with embedded migrations that are applied on startup.
- `web/`: a React SPA built with Vite, TanStack Router and TanStack Query.

## Requirements

- Go 1.26 or later
- Node.js and [pnpm](https://pnpm.io)
- Docker (for Postgres)
- A Discord application with a bot, created in the [Developer Portal](https://discord.com/developers/applications)

## Getting started

### 1. Discord application

In the Developer Portal:

1. Under **OAuth2**, add the redirect `http://localhost:8080/auth/callback`.
2. Under **Bot**, enable the privileged **Message Content** intent (only the AI assistant uses it).
3. Copy the client ID, the client secret and the bot token.

### 2. Configuration

Copy `.env.example` to `.env` in the repo root and fill in the Discord credentials:

```bash
cp .env.example .env
```

To enable the AI assistant, generate an encryption key and put it in `AI_ENCRYPTION_KEY`. Without it the feature stays disabled:

```bash
openssl rand -base64 32
```

### 3. Database

```bash
docker compose up -d
```

On macOS with Apple silicon you can also use `scripts/db-mac.sh`, which relies on Apple `container`.

### 4. Backend

```bash
cd backend
go run ./cmd/api
```

The API listens on `:8080`.

### 5. Frontend

```bash
cd web
pnpm install
pnpm dev
```

The dashboard is served at <http://localhost:5173>, with `/api` and `/auth` proxied to the backend.

## Tests

```bash
cd backend
go test ./...
```

The tests in `internal/store` need `TEST_DATABASE_URL`, pointing to an empty database that is different from the development one. Without it they are skipped.

## Production

- Set `COOKIE_SECURE=true` and serve everything over HTTPS.
- Set `DISCORD_REDIRECT_URL` and `FRONTEND_URL` to your own domains.
- Keep `AI_ALLOW_PRIVATE_URLS=false`: with `true` the assistant could connect to addresses on your internal network.
- If you lose or change `AI_ENCRYPTION_KEY`, admins will have to enter their LLM API keys again.

## License

[GNU AGPL-3.0](LICENSE). If you offer a modified version of Askord as a service, you must publish your changes under the same license.
