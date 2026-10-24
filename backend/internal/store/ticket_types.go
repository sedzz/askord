package store

import (
	"context"
	"errors"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// MaxTicketTypes es el tope de tipos por servidor (el máximo de opciones de un desplegable de Discord).
const MaxTicketTypes = 25

var (
	ErrTypeNameTaken = errors.New("ya existe un tipo con ese nombre")
	ErrTooManyTypes  = errors.New("se alcanzó el máximo de tipos")
	ErrLastType      = errors.New("un servidor necesita al menos un tipo")
	ErrBadOrder      = errors.New("el orden no coincide con los tipos existentes")
)

// TicketType es una clase de ticket que define cada servidor.
type TicketType struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Emoji       string `json:"emoji"`
	Description string `json:"description"`
	// Vacíos: se usa la categoría y el rol de staff por defecto del servidor.
	CategoryID  string `json:"categoryId"`
	StaffRoleID string `json:"staffRoleId"`
	Position    int    `json:"position"`
	// Formulario que se muestra al abrir un ticket de este tipo (puede estar vacío).
	Questions []Question `json:"questions"`
}

// MaxQuestions es el máximo de campos de un formulario de Discord.
const MaxQuestions = 5

// Question es una pregunta del formulario de un tipo.
type Question struct {
	Label    string `json:"label"`
	Required bool   `json:"required"`
	// Long: respuesta larga (párrafo) en lugar de una línea.
	Long bool `json:"long"`
}

// TicketTypeInput son los campos editables de un tipo.
type TicketTypeInput struct {
	Name        string
	Emoji       string
	Description string
	CategoryID  string
	StaffRoleID string
	Questions   []Question
}

const typeColumns = `id, name, emoji, description, COALESCE(category_id, ''), COALESCE(staff_role_id, ''), position, questions`

func scanType(row pgx.Row) (TicketType, error) {
	var t TicketType
	err := row.Scan(&t.ID, &t.Name, &t.Emoji, &t.Description, &t.CategoryID, &t.StaffRoleID, &t.Position, &t.Questions)
	if t.Questions == nil {
		t.Questions = []Question{}
	}
	return t, err
}

// questionsOrEmpty evita guardar null: sin preguntas, una lista vacía.
func questionsOrEmpty(q []Question) []Question {
	if q == nil {
		return []Question{}
	}
	return q
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (s *Store) ListTicketTypes(ctx context.Context, guildID string) ([]TicketType, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+typeColumns+` FROM ticket_types WHERE guild_id = $1 ORDER BY position, id`, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []TicketType{}
	for rows.Next() {
		t, err := scanType(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) CreateTicketType(ctx context.Context, guildID string, in TicketTypeInput) (TicketType, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TicketType{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var count, next int
	if err := tx.QueryRow(ctx,
		`SELECT count(*), COALESCE(max(position) + 1, 0) FROM ticket_types WHERE guild_id = $1`, guildID).
		Scan(&count, &next); err != nil {
		return TicketType{}, err
	}
	if count >= MaxTicketTypes {
		return TicketType{}, ErrTooManyTypes
	}

	t, err := scanType(tx.QueryRow(ctx, `
		INSERT INTO ticket_types (guild_id, name, emoji, description, category_id, staff_role_id, position, questions)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING `+typeColumns,
		guildID, in.Name, in.Emoji, in.Description, nullable(in.CategoryID), nullable(in.StaffRoleID), next,
		questionsOrEmpty(in.Questions)))
	if isUniqueViolation(err) {
		return TicketType{}, ErrTypeNameTaken
	}
	if err != nil {
		return TicketType{}, err
	}
	return t, tx.Commit(ctx)
}

func (s *Store) UpdateTicketType(ctx context.Context, guildID string, id int64, in TicketTypeInput) (TicketType, error) {
	t, err := scanType(s.pool.QueryRow(ctx, `
		UPDATE ticket_types
		SET name = $3, emoji = $4, description = $5, category_id = $6, staff_role_id = $7, questions = $8
		WHERE guild_id = $1 AND id = $2
		RETURNING `+typeColumns,
		guildID, id, in.Name, in.Emoji, in.Description, nullable(in.CategoryID), nullable(in.StaffRoleID),
		questionsOrEmpty(in.Questions)))
	if errors.Is(err, pgx.ErrNoRows) {
		return TicketType{}, ErrNotFound
	}
	if isUniqueViolation(err) {
		return TicketType{}, ErrTypeNameTaken
	}
	return t, err
}

// DeleteTicketType borra un tipo, salvo que sea el último del servidor.
func (s *Store) DeleteTicketType(ctx context.Context, guildID string, id int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM ticket_types WHERE guild_id = $1`, guildID).Scan(&count); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM ticket_types WHERE guild_id = $1 AND id = $2`, guildID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if count <= 1 {
		return ErrLastType
	}
	return tx.Commit(ctx)
}

// ReorderTicketTypes fija el orden: ids debe contener exactamente los tipos del servidor.
func (s *Store) ReorderTicketTypes(ctx context.Context, guildID string, ids []int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	rows, err := tx.Query(ctx, `SELECT id FROM ticket_types WHERE guild_id = $1`, guildID)
	if err != nil {
		return err
	}
	existing, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return err
	}
	a, b := slices.Clone(existing), slices.Clone(ids)
	slices.Sort(a)
	slices.Sort(b)
	if !slices.Equal(a, b) {
		return ErrBadOrder
	}

	for position, id := range ids {
		if _, err := tx.Exec(ctx,
			`UPDATE ticket_types SET position = $3 WHERE guild_id = $1 AND id = $2`, guildID, id, position); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// EnsureDefaultTicketType crea el tipo "General" si el servidor aún no tiene ninguno.
func (s *Store) EnsureDefaultTicketType(ctx context.Context, guildID string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO ticket_types (guild_id, name, emoji, description)
		SELECT $1, 'General', '📩', 'Abre un ticket y el equipo te atenderá.'
		WHERE NOT EXISTS (SELECT 1 FROM ticket_types WHERE guild_id = $1)`, guildID)
	return err
}

// GetTicketType devuelve un tipo del servidor, o ErrNotFound.
func (s *Store) GetTicketType(ctx context.Context, guildID string, id int64) (TicketType, error) {
	t, err := scanType(s.pool.QueryRow(ctx,
		`SELECT `+typeColumns+` FROM ticket_types WHERE guild_id = $1 AND id = $2`, guildID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return TicketType{}, ErrNotFound
	}
	return t, err
}
