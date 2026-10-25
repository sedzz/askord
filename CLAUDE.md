# CLAUDE.md

Este archivo da orientación a Claude Code (claude.ai/code) para trabajar con el código de este repositorio.

## Resumen

Askord es un bot de tickets para Discord más un panel web para configurarlo. Es un monorepo con un backend en Go (`backend/`, módulo `github.com/sedzz/askord`) que ejecuta el bot de Discord **y** la API HTTP en un solo proceso, y una SPA en React (`web/`). Los textos de cara al usuario, los comentarios y la documentación están en español; mantén esa convención.

## Comandos

Postgres (imagen pgvector) con Docker; copia `.env.example` a `.env` en la raíz del repo y completa las credenciales de Discord (`godotenv` carga `../.env` o `.env`):

```bash
docker compose up -d
```

En macOS con Apple silicon se puede usar Apple `container` en vez de Docker: `scripts/db-mac.sh` (idempotente) levanta el mismo Postgres. En Windows sigue siendo `docker compose`.

Backend (ejecutar desde `backend/`):

```bash
go run ./cmd/api                          # API en :8080 + bot de Discord
go test ./...
go test ./internal/bot -run TestName      # un solo test
```

Frontend (ejecutar desde `web/`, usa pnpm):

```bash
pnpm dev      # Vite en :5173, con proxy de /api y /auth hacia :8080
pnpm build    # tsc -b && vite build
pnpm lint     # oxlint
```

Las migraciones son SQL en formato goose en `backend/db/migrations/` (`-- +goose Up`) y van embebidas en el binario (`backend/db/embed.go`): la API las aplica sola al arrancar (`store.Migrate`, con candado de Postgres por si hay varias copias). Las nuevas llevan el siguiente número secuencial. Los tests con base de datos (`internal/store`) usan `TEST_DATABASE_URL`, una base vacía distinta de la de desarrollo; sin ella se omiten.

## Arquitectura

- `backend/cmd/api/main.go` conecta todo: `store` (pool de pgx) → `bot` (discordgo, solo el intent `Guilds`) → handlers de `auth` y `dashboard` sobre un router chi. Rutas públicas: `/healthz`, `/auth/*`; todo lo que cuelga de `/api` pasa por `auth.Require` (cookie de sesión, login con OAuth de Discord).
- `internal/store` es el único paquete que habla con Postgres; los demás llaman a sus métodos.
- `internal/ai` conecta con LLMs compatibles con la API de OpenAI: la URL y la API key las pone el admin de cada servidor (`/api/guilds/{id}/ai`). La key se guarda cifrada con AES-GCM (`AI_ENCRYPTION_KEY`; sin ella la IA queda desactivada) y no se devuelve nunca. El cliente bloquea direcciones locales y privadas (SSRF) salvo con `AI_ALLOW_PRIVATE_URLS=true`. Además guarda el comportamiento: interruptor general y pautas del servidor (`ai_settings.enabled/guidelines`, `PUT .../ai/behavior`) y, por tipo de ticket, `ai_enabled`, `ai_instructions` y `ai_escalation`. Los textos se normalizan con `ai.CleanText` y tienen límites de longitud (`ai.Max*`). `ai.Client.Chat` llama al modelo (con la herramienta `escalar_a_staff`; los fallos son `*ai.Error` con un `Kind`) y `ai.BuildMessages` monta el prompt: reglas fijas + textos del admin en el mensaje de sistema, y todo lo que escriben los usuarios (formulario incluido) siempre como mensajes de usuario. En cada ticket, `tickets.ai_state` (`active`, `escalated` o `paused`) y `ai_replies` llevan el estado del asistente: `store.ClaimAIReply` reserva una respuesta de forma atómica (con tope) y `store.SetAIState` lo saca de `active` una sola vez. Todavía no está conectado al bot.
- Los handlers HTTP de `internal/dashboard` tienen tanto `store` como `bot`: la configuración vive en la base de datos, pero las acciones que tocan Discord (crear categoría/rol/canal, publicar el panel de tickets, datos del servidor cacheados ~1 min) pasan por `bot.Bot`.
- `internal/bot` gestiona los eventos del gateway y las interacciones de Discord: panel con un botón por tipo de ticket, formularios (modales) por tipo, canales/hilos de ticket, cierre y transcripciones (`transcript.go`). El nombre y la configuración del tipo se copian al ticket (migración 00010), así que editar un tipo no reescribe el historial.
- Las transcripciones (`ticket_messages`) y las respuestas de formulario (`tickets.answers`) se exportan con `internal/dashboard/export.go`. La retención ya está implementada (las notas de diseño están en `docs/`, que no se versiona): `internal/cleanup` purga el contenido 90 días después de `closed_at` (al arrancar y cada 6 h, no con `pg_cron`) y hay un botón de borrado manual. `tickets.content_purged_at` marca los tickets purgados. Las filas de tickets se conservan para que las estadísticas del panel no cambien.
- Frontend: Vite + React 19, TanStack Router (rutas basadas en archivos en `web/src/routes`; `routeTree.gen.ts` es generado, no lo edites) y TanStack Query. Cada archivo `web/src/lib/*.ts` junta un tipo de TypeScript con sus `queryOptions`/fetch para un recurso de la API. Las primitivas reutilizables están en `components/ui/`, cada una con su propio `.css`; `components/discord/` imita la interfaz de Discord para las vistas previas de paneles, tickets y formularios.
