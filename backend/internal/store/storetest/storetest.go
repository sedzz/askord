// Package storetest da a los tests de otros paquetes una base de datos limpia y migrada.
package storetest

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sedzz/askord/internal/store"
)

// urls recuerda la base de datos de cada store, para que Exec pueda ejecutar SQL sobre ella.
var urls sync.Map // *store.Store -> string

// Exec ejecuta SQL directamente sobre la base de datos de st: sirve para preparar situaciones
// que la API del store no permite, como envejecer un ticket.
func Exec(t *testing.T, st *store.Store, sql string, args ...any) {
	t.Helper()
	url, ok := urls.Load(st)
	if !ok {
		t.Fatal("el store no viene de storetest.New")
	}
	pool, err := pgxpool.New(context.Background(), url.(string))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

// New crea una base de datos nueva a partir de TEST_DATABASE_URL, le aplica las migraciones
// y la borra al terminar el test. Sin la variable, el test se omite.
func New(t *testing.T) *store.Store {
	t.Helper()
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

	name := fmt.Sprintf("askord_test_%d", time.Now().UnixNano())
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
	st, err := store.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	urls.Store(st, u.String())
	t.Cleanup(func() { urls.Delete(st) })
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	return st
}
