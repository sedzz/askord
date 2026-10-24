package store

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"github.com/sedzz/askord/db"
)

// Migrate aplica las migraciones pendientes. Un candado de Postgres hace que, con varias
// copias de la API arrancando a la vez, solo una las ejecute y las demás esperen.
func (s *Store) Migrate(ctx context.Context) error {
	fsys, err := fs.Sub(db.Migrations, "migrations")
	if err != nil {
		return err
	}
	sqlDB := stdlib.OpenDBFromPool(s.pool)
	defer sqlDB.Close() //nolint:errcheck

	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, fsys, goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("preparar migraciones: %w", err)
	}
	results, err := provider.Up(ctx)
	for _, r := range results {
		slog.Info("migración aplicada", "version", r.Source.Version, "file", r.Source.Path)
	}
	if err != nil {
		return fmt.Errorf("aplicar migraciones: %w", err)
	}
	return nil
}
