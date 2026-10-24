package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"

	"github.com/sedzz/askord/internal/auth"
	"github.com/sedzz/askord/internal/config"
	"github.com/sedzz/askord/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fallo al arrancar", "err", err)
		os.Exit(1)
	}
}

func run() error {
	// .env en la raiz del repo (opcional; en produccion vienen del entorno).
	_ = godotenv.Load("../.env", ".env")

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer st.Close()

	authH := auth.New(cfg, st)

	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	r.Route("/auth", func(r chi.Router) {
		r.Get("/login", authH.Login)
		r.Get("/callback", authH.Callback)
		r.Post("/logout", authH.Logout)
	})

	r.Route("/api", func(r chi.Router) {
		r.Use(authH.Require)
		r.Get("/me", authH.Me)
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		slog.Info("api escuchando", "addr", cfg.HTTPAddr)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
	return nil
}
