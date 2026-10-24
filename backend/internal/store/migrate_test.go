package store

import (
	"context"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sedzz/askord/db"
)

// TestMigrate aplica todas las migraciones sobre una base de datos nueva y comprueba que
// repetirlas no hace nada. Crea y borra su propia base a partir de TEST_DATABASE_URL.
func TestMigrate(t *testing.T) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL no definida")
	}
	ctx := context.Background()

	admin, err := pgxpool.New(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close) // se registra primero, así se ejecuta el último

	name := fmt.Sprintf("askord_migrate_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, `CREATE DATABASE `+name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, `DROP DATABASE `+name+` WITH (FORCE)`); err != nil {
			t.Errorf("borrar %s: %v", name, err)
		}
	})

	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	st, err := New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	for range 2 { // la segunda vez no debe hacer nada ni fallar
		if err := st.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}

	files, err := fs.Glob(db.Migrations, "migrations/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	var applied int
	if err := st.pool.QueryRow(ctx, `SELECT count(*) FROM goose_db_version WHERE version_id > 0`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != len(files) {
		t.Errorf("aplicadas %d migraciones, hay %d archivos", applied, len(files))
	}

	var col string
	err = st.pool.QueryRow(ctx, `SELECT column_name FROM information_schema.columns
		WHERE table_name = 'tickets' AND column_name = 'content_purged_at'`).Scan(&col)
	if err != nil || !strings.EqualFold(col, "content_purged_at") {
		t.Errorf("falta tickets.content_purged_at (err %v)", err)
	}
}
