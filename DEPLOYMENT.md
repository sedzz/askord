# Deployment

Askord can be deployed on any Linux server with Docker. Nothing is published on the host: the only way in is a [Cloudflare Tunnel](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/), so the server's IP is never exposed and no ports need to be opened.

```
Internet ──▶ Cloudflare ──▶ cloudflared ──▶ web (nginx) ──▶ backend ──▶ postgres
                                              └─ serves the SPA, proxies /api and /auth
```

Everything private (server address, SSH key, Discord credentials) lives in GitHub Secrets and in a `.env` on the server. None of it is in this repository.

## One-time setup

1. **Cloudflare**: Zero Trust → Networks → Tunnels → create a tunnel and copy its token. Add a public hostname pointing to `http://web:8080`.
2. **Discord**: in the Developer Portal add `https://<your-domain>/auth/callback` as an OAuth2 redirect.
3. **Server**: create a directory `askord` in the home of the deploy user and put a `.env` in it, based on `.env.production.example`.
4. **SSH key**: generate a key used only for deploying (`ssh-keygen -t ed25519 -f deploy_key -N ""`) and add the public part to the deploy user's `authorized_keys`.
5. **GitHub**: create an environment named `production` (restrict it to the `main` branch) with these secrets:

   | Secret | Value |
   | --- | --- |
   | `VPS_HOST` | server address |
   | `VPS_USER` | deploy user |
   | `VPS_SSH_KEY` | the private deploy key |
   | `VPS_KNOWN_HOSTS` | output of `ssh-keyscan -t ed25519 <host>` |

6. **GHCR**: after the first run, set the `askord-backend` and `askord-web` packages to public (the images contain no secrets), or log in to `ghcr.io` on the server with a read-only token.

## How it works

`.github/workflows/deploy.yml` runs after `CI` succeeds on a push to `main` (never for pull requests). It builds and pushes both images to GHCR and then connects over SSH to copy `docker-compose.prod.yml` and run `docker compose pull && up -d`. It can also be launched by hand with *Run workflow*.

Database migrations are applied by the backend on startup.
