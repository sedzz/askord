# Askord

Bot de tickets para Discord con un panel web para configurarlo.

## Qué hace

- **Panel de tickets** en Discord con un botón por tipo de ticket.
- **Tipos de ticket** configurables: nombre, emoji, categoría, rol de staff y un formulario propio con preguntas. Se reordenan arrastrando.
- **Transcripciones** de cada ticket al cerrarlo, con exportación. El contenido se purga solo 90 días después del cierre, y también se puede borrar a mano.
- **Asistente de IA** opcional: cada servidor conecta su propio LLM (cualquier proveedor compatible con la API de OpenAI) y define pautas generales y por tipo de ticket. El asistente responde en los tickets y avisa al staff cuando no puede resolverlo.
- **Panel web** con login de Discord, estadísticas y configuración de cada servidor.

## Cómo está hecho

Monorepo con dos partes:

- `backend/`: Go. Un solo proceso ejecuta el bot de Discord y la API HTTP. Usa Postgres con migraciones embebidas que se aplican al arrancar.
- `web/`: SPA en React con Vite, TanStack Router y TanStack Query.

## Requisitos

- Go 1.26 o superior
- Node.js y [pnpm](https://pnpm.io)
- Docker (para Postgres)
- Una aplicación de Discord con bot, creada en el [Developer Portal](https://discord.com/developers/applications)

## Puesta en marcha

### 1. Aplicación de Discord

En el Developer Portal:

1. En **OAuth2**, añade la redirección `http://localhost:8080/auth/callback`.
2. En **Bot**, activa el intent privilegiado **Message Content** (solo lo usa el asistente de IA).
3. Copia el client ID, el client secret y el token del bot.

### 2. Configuración

Copia `.env.example` a `.env` en la raíz del repo y rellena las credenciales de Discord:

```bash
cp .env.example .env
```

Para activar el asistente de IA, genera una clave de cifrado y ponla en `AI_ENCRYPTION_KEY`. Sin ella la función queda desactivada:

```bash
openssl rand -base64 32
```

### 3. Base de datos

```bash
docker compose up -d
```

En macOS con Apple silicon también se puede usar `scripts/db-mac.sh` con Apple `container`.

### 4. Backend

```bash
cd backend
go run ./cmd/api
```

La API escucha en `:8080`.

### 5. Frontend

```bash
cd web
pnpm install
pnpm dev
```

El panel queda en <http://localhost:5173>, con proxy de `/api` y `/auth` hacia el backend.

## Tests

```bash
cd backend
go test ./...
```

Los tests de `internal/store` necesitan `TEST_DATABASE_URL`, apuntando a una base vacía distinta de la de desarrollo. Sin ella se omiten.

## Producción

- Pon `COOKIE_SECURE=true` y sirve todo por HTTPS.
- Ajusta `DISCORD_REDIRECT_URL` y `FRONTEND_URL` a tus dominios.
- Mantén `AI_ALLOW_PRIVATE_URLS=false`: con `true` el asistente podría conectarse a direcciones de tu red interna.
- Si pierdes o cambias `AI_ENCRYPTION_KEY`, los admins tendrán que volver a introducir las API keys de sus LLM.

## Licencia

[GNU AGPL-3.0](LICENSE). Si ofreces una versión modificada de Askord como servicio, debes publicar tus cambios bajo la misma licencia.
