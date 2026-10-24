// Package cleanup ejecuta en segundo plano la limpieza periódica de datos caducados.
package cleanup

import (
	"context"
	"log/slog"
	"time"

	"github.com/sedzz/askord/internal/store"
)

const (
	// Cada cuánto se repite la limpieza. Si la API estuvo apagada, al volver recupera lo atrasado.
	interval = 6 * time.Hour
	// Tickets que se limpian por vuelta, para no bloquear la base de datos.
	batchSize = 500
)

// Run limpia al arrancar y luego cada pocas horas, hasta que se cancele ctx.
func Run(ctx context.Context, st *store.Store) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		once(ctx, st, time.Now())
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func once(ctx context.Context, st *store.Store, now time.Time) {
	purgeTranscripts(ctx, st, now)
	if err := st.DeleteExpiredSessions(ctx); err != nil && ctx.Err() == nil {
		slog.Error("limpiar sesiones caducadas", "err", err)
	}
}

// purgeTranscripts repite por lotes hasta no encontrar más tickets caducados.
func purgeTranscripts(ctx context.Context, st *store.Store, now time.Time) {
	cutoff := now.Add(-store.TranscriptRetention)
	total := 0
	for ctx.Err() == nil {
		n, err := st.PurgeExpiredTranscripts(ctx, cutoff, batchSize)
		if err != nil {
			if ctx.Err() == nil {
				slog.Error("borrar transcripciones caducadas", "err", err)
			}
			break
		}
		total += n
		if n < batchSize {
			break
		}
	}
	if total > 0 {
		slog.Info("transcripciones caducadas eliminadas", "tickets", total)
	}
}
