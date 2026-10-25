package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// Estados del asistente de IA en un ticket.
const (
	AIStateActive    = "active"    // responde
	AIStateEscalated = "escalated" // pasó el ticket a una persona
	AIStatePaused    = "paused"    // se calló porque escribió el staff
)

// ClaimResult es el resultado de pedir permiso para que el asistente responda.
type ClaimResult int

const (
	ClaimGranted      ClaimResult = iota // puede responder; ya se contó la respuesta
	ClaimLimitReached                    // alcanzó el tope de respuestas del ticket
	ClaimNotActive                       // el ticket está cerrado, escalado, pausado o no existe
)

// ClaimAIReply reserva una respuesta del asistente en el ticket de ese canal. Es atómico: con
// varios mensajes llegando a la vez nunca se reparten más respuestas que el tope max.
func (s *Store) ClaimAIReply(ctx context.Context, channelID string, max int) (ClaimResult, error) {
	var replies int
	err := s.pool.QueryRow(ctx, `
		UPDATE tickets SET ai_replies = ai_replies + 1
		WHERE channel_id = $1 AND status = 'open' AND ai_state = 'active' AND ai_replies < $2
		RETURNING ai_replies`, channelID, max).Scan(&replies)
	if err == nil {
		return ClaimGranted, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return ClaimNotActive, err
	}

	// No se concedió: se distingue el tope de las demás razones.
	var (
		status, state string
		used          int
	)
	err = s.pool.QueryRow(ctx,
		`SELECT status, ai_state, ai_replies FROM tickets WHERE channel_id = $1`, channelID).Scan(&status, &state, &used)
	if errors.Is(err, pgx.ErrNoRows) {
		return ClaimNotActive, nil
	}
	if err != nil {
		return ClaimNotActive, err
	}
	if status == "open" && state == AIStateActive && used >= max {
		return ClaimLimitReached, nil
	}
	return ClaimNotActive, nil
}

// SetAIState saca al asistente del estado activo (a escalado o pausado) en el ticket de ese
// canal. Devuelve false si no cambió nada: el ticket no existe o el asistente ya no estaba
// activo. Así, quien lo llame solo avisa una vez aunque lleguen dos peticiones a la vez.
func (s *Store) SetAIState(ctx context.Context, channelID, state string) (bool, error) {
	if state != AIStateEscalated && state != AIStatePaused {
		return false, errors.New("estado de IA no válido")
	}
	tag, err := s.pool.Exec(ctx,
		`UPDATE tickets SET ai_state = $2 WHERE channel_id = $1 AND ai_state = 'active'`, channelID, state)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}
