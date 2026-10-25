# Contributing to Askord

Thanks for your interest in Askord. Bug reports, ideas and pull requests are welcome.

## Before you start

- For a bug, open an issue with the steps to reproduce it and what you expected to happen.
- For a new feature or a big change, open an issue first so we can agree on the approach before you spend time on it.
- For security problems, **do not open a public issue**. Follow [SECURITY.md](SECURITY.md).

## Development setup

The [README](README.md) explains how to create the Discord application, configure `.env`, start Postgres and run the backend and the frontend.

## Before opening a pull request

Make sure these pass; they are the same checks the CI runs:

```bash
cd backend
go vet ./...
go test ./...
```

```bash
cd web
pnpm lint
pnpm build
```

The tests in `internal/store` need a database. Set `TEST_DATABASE_URL` to an empty database that is different from the development one; without it they are skipped.

## Guidelines

- **Keep pull requests small and focused.** One change per pull request is easier to review.
- **Add tests** for new behavior and bug fixes, next to the code they cover.
- **Migrations** are goose SQL files in `backend/db/migrations/`. Use the next sequential number and never edit a migration that is already merged.
- **Only `internal/store` talks to Postgres.** The other packages call its methods.
- **Never commit secrets**: tokens, API keys or `.env` files. Use `.env.example` for new settings, with a comment explaining them.
- **Language.** Documentation is written in English. The texts users see (the bot and the dashboard) and the code comments are currently in Spanish; match the surrounding code until they are translated.
- **Commit messages** are in English, short, and written in the imperative (for example, "Add a retry to the transcript export").

## License

Askord is licensed under the [GNU AGPL-3.0](LICENSE). By submitting a contribution you agree that it is released under the same license.
