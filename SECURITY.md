# Security policy

## How to report a vulnerability

If you find a security flaw in Askord, **do not open a public issue**. Use GitHub's private reporting: the repository's **Security** tab → **Report a vulnerability**.

If you can, include:

- What you found and what its impact is.
- Steps to reproduce it.
- The version or commit where you saw it.

I will try to answer within a few days. Please do not disclose the flaw publicly until a fix is available.

## What is in scope

- Access to other servers' data or tickets without permission.
- Authentication or session flaws in the dashboard.
- Leaks of the LLM API keys (stored encrypted) or of Discord tokens.
- Bypassing the SSRF protection of the LLM client.
- Injection (SQL, XSS) in the API or the dashboard.

## Deployment

If you host Askord yourself, keep `COOKIE_SECURE=true` behind HTTPS and `AI_ALLOW_PRIVATE_URLS=false`, and keep `.env` and `AI_ENCRYPTION_KEY` out of the repository.
