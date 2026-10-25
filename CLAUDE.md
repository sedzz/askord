# CLAUDE.md

This file gives Claude Code (claude.ai/code) guidance for working with the code in this repository.

## Overview

Askord is a Discord ticket bot plus a web dashboard to configure it. It is a monorepo with a Go backend (`backend/`, module `github.com/sedzz/askord`) that runs the Discord bot **and** the HTTP API in a single process, and a React SPA (`web/`). Documentation is written in English. The user-facing texts (bot and dashboard) and the code comments are currently in Spanish; keep that convention until they are translated.

## Commands

Postgres (pgvector image) with Docker; copy `.env.example` to `.env` in the repo root and fill in the Discord credentials (`godotenv` loads `../.env` or `.env`):

```bash
docker compose up -d
```

On macOS with Apple silicon you can use Apple `container` instead of Docker: `scripts/db-mac.sh` (idempotent) starts the same Postgres. On Windows it is still `docker compose`.

Backend (run from `backend/`):

```bash
go run ./cmd/api                          # API on :8080 + Discord bot
go test ./...
go test ./internal/bot -run TestName      # a single test
```

Frontend (run from `web/`, uses pnpm):

```bash
pnpm dev      # Vite on :5173, proxying /api and /auth to :8080
pnpm build    # tsc -b && vite build
pnpm lint     # oxlint
```

Migrations are goose-format SQL in `backend/db/migrations/` (`-- +goose Up`) and are embedded in the binary (`backend/db/embed.go`): the API applies them on startup (`store.Migrate`, with a Postgres lock in case several copies run). New ones take the next sequential number. Database tests (`internal/store`) use `TEST_DATABASE_URL`, an empty database different from the development one; without it they are skipped.

## Architecture

- `backend/cmd/api/main.go` wires everything together: `store` (pgx pool) → `bot` (discordgo; intents `Guilds`, `GuildMessages` and the privileged `MessageContent`, the latter only for the AI assistant) → `auth` and `dashboard` handlers on a chi router. Public routes: `/healthz`, `/auth/*`; everything under `/api` goes through `auth.Require` (session cookie, Discord OAuth login).
- `internal/store` is the only package that talks to Postgres; the others call its methods.
- `internal/ai` connects to LLMs compatible with the OpenAI API: the URL and API key are set by each server's admin (`/api/guilds/{id}/ai`). The key is stored encrypted with AES-GCM (`AI_ENCRYPTION_KEY`; without it the AI is disabled) and is never returned. The client blocks local and private addresses (SSRF) unless `AI_ALLOW_PRIVATE_URLS=true`. It also stores the behavior: a global switch and the server guidelines (`ai_settings.enabled/guidelines`, `PUT .../ai/behavior`) and, per ticket type, `ai_enabled`, `ai_instructions` and `ai_escalation`. Texts are normalized with `ai.CleanText` and have length limits (`ai.Max*`). `ai.Client.Chat` calls the model (with the `escalar_a_staff` tool; failures are `*ai.Error` with a `Kind`) and `ai.BuildMessages` builds the prompt: fixed rules + the admin's texts in the system message, and everything users write (form included) always as user messages. In each ticket, `tickets.ai_state` (`active`, `escalated` or `paused`) and `ai_replies` hold the assistant's state: `store.ClaimAIReply` reserves a reply atomically (with a cap) and `store.SetAIState` takes it out of `active` exactly once. The assistant lives in `internal/bot/assistant.go` (enabled with `Bot.EnableAI`): it answers messages in tickets whose type has AI, waits ~3 s after the last message, mentions nobody, and hands the ticket to the staff (`escalar_a_staff` tool, the `ai.MaxReplies` cap or a provider failure). The notice with the reason goes to the private alerts channel (`ai_settings.alerts_channel_id`, which the bot can create with `CreateStaffChannel`); the user only sees a neutral message in their ticket, and without a channel the role is mentioned in the ticket with no reason. It pauses if a staff member who did not open the ticket writes, and drops its reply if the ticket changed while the model was thinking.
- The HTTP handlers in `internal/dashboard` have both `store` and `bot`: the configuration lives in the database, but the actions that touch Discord (creating a category/role/channel, publishing the ticket panel, guild data cached for ~1 min) go through `bot.Bot`.
- `internal/bot` handles gateway events and Discord interactions: a panel with one button per ticket type, forms (modals) per type, ticket channels/threads, closing and transcripts (`transcript.go`). The type's name and configuration are copied into the ticket (migration 00010), so editing a type does not rewrite history.
- Transcripts (`ticket_messages`) and form answers (`tickets.answers`) are exported with `internal/dashboard/export.go`. Retention is implemented: `internal/cleanup` purges the content 90 days after `closed_at` (on startup and every 6 h, not with `pg_cron`) and there is a manual delete button. `tickets.content_purged_at` marks purged tickets. Ticket rows are kept so the dashboard statistics do not change.
- Frontend: Vite + React 19, TanStack Router (file-based routes in `web/src/routes`; `routeTree.gen.ts` is generated, do not edit it) and TanStack Query. Each `web/src/lib/*.ts` file bundles a TypeScript type with its `queryOptions`/fetch for an API resource. Components are grouped by feature in `web/src/components/` (`ai/`, `layout/`, `tickets/`, `transcripts/`, `settings/`, `dashboard/`). Reusable primitives are in `components/ui/`, each with its own `.css`; `components/discord/` imitates the Discord interface for the previews of panels, tickets and forms.
