package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type Note struct {
	ID            string    `json:"id"`
	OwnerID       string    `json:"ownerId"`
	Title         string    `json:"title"`
	Body          string    `json:"body"`
	DocumentID    *string   `json:"documentId"`
	QuoteLocation *string   `json:"quoteLocation"`
	Tags          []string  `json:"tags"`
	Version       int       `json:"version"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type NoteSummary struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Snippet   string    `json:"snippet"`
	Tags      []string  `json:"tags"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type NoteInput struct {
	Title         string   `json:"title"`
	Body          string   `json:"body"`
	DocumentID    *string  `json:"documentId"`
	QuoteLocation *string  `json:"quoteLocation"`
	Tags          []string `json:"tags"`
}

// Права доступа в одном месте. Во всех запросах ниже $1 — id текущего пользователя.
// Читать может владелец или тот, с кем поделились; редактировать — владелец или editor.
const (
	noteCanRead = `(n.owner_id = $1 OR EXISTS (
		SELECT 1 FROM shares s WHERE s.resource_type = 'note' AND s.resource_id = n.id AND s.user_id = $1))`
	noteCanEdit = `(n.owner_id = $1 OR EXISTS (
		SELECT 1 FROM shares s WHERE s.resource_type = 'note' AND s.resource_id = n.id AND s.user_id = $1
		AND s.role = 'editor'))`
	noteTagsAgg = `COALESCE((SELECT array_agg(t.name ORDER BY t.name)
		FROM note_tags nt JOIN tags t ON t.id = nt.tag_id WHERE nt.note_id = n.id), '{}')`
)

// ListNotes возвращает заметки пользователя, свежие сверху. tag == "" — без фильтра.
func (s *Store) ListNotes(ctx context.Context, userID, tag string) ([]NoteSummary, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT n.id, n.title, left(n.body, 200), `+noteTagsAgg+`, n.updated_at
		FROM notes n
		WHERE `+noteCanRead+`
		  AND ($2::text = '' OR EXISTS (
		      SELECT 1 FROM note_tags nt JOIN tags t ON t.id = nt.tag_id
		      WHERE nt.note_id = n.id AND t.name = $2))
		ORDER BY n.updated_at DESC
		LIMIT 500`, userID, tag)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (NoteSummary, error) {
		var n NoteSummary
		err := r.Scan(&n.ID, &n.Title, &n.Snippet, &n.Tags, &n.UpdatedAt)
		return n, err
	})
}

func (s *Store) GetNote(ctx context.Context, userID, noteID string) (Note, error) {
	return getNote(ctx, s.pool, userID, noteID)
}

// querier — общий интерфейс пула и транзакции, чтобы getNote работал внутри tx.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func getNote(ctx context.Context, q querier, userID, noteID string) (Note, error) {
	var n Note
	err := q.QueryRow(ctx, `
		SELECT n.id, n.owner_id, n.title, n.body, n.document_id, n.quote_location,
		       `+noteTagsAgg+`, n.version, n.created_at, n.updated_at
		FROM notes n
		WHERE n.id = $2 AND `+noteCanRead, userID, noteID,
	).Scan(&n.ID, &n.OwnerID, &n.Title, &n.Body, &n.DocumentID, &n.QuoteLocation,
		&n.Tags, &n.Version, &n.CreatedAt, &n.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Note{}, ErrNotFound
	}
	return n, err
}

func (s *Store) CreateNote(ctx context.Context, userID string, in NoteInput) (Note, error) {
	var note Note
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var id string
		if err := tx.QueryRow(ctx, `
			INSERT INTO notes (owner_id, title, body, document_id, quote_location)
			VALUES ($1, $2, $3, $4, $5) RETURNING id`,
			userID, in.Title, in.Body, in.DocumentID, in.QuoteLocation,
		).Scan(&id); err != nil {
			return err
		}
		if err := setNoteTags(ctx, tx, id, userID, in.Tags); err != nil {
			return err
		}
		var err error
		note, err = getNote(ctx, tx, userID, id)
		return err
	})
	return note, err
}

// UpdateNote сохраняет заметку, только если её версия всё ещё равна expectedVersion.
// Иначе ErrConflict: заметку успели изменить на другом устройстве.
func (s *Store) UpdateNote(ctx context.Context, userID, noteID string, expectedVersion int, in NoteInput) (Note, error) {
	var note Note
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var ownerID string
		err := tx.QueryRow(ctx, `
			UPDATE notes n
			SET title = $3, body = $4, document_id = $5, quote_location = $6,
			    version = version + 1, updated_at = now()
			WHERE n.id = $2 AND n.version = $7 AND `+noteCanEdit+`
			RETURNING n.owner_id`,
			userID, noteID, in.Title, in.Body, in.DocumentID, in.QuoteLocation, expectedVersion,
		).Scan(&ownerID)
		if errors.Is(err, pgx.ErrNoRows) {
			return s.explainFailedUpdate(ctx, tx, userID, noteID)
		}
		if err != nil {
			return err
		}
		if err := setNoteTags(ctx, tx, noteID, ownerID, in.Tags); err != nil {
			return err
		}
		note, err = getNote(ctx, tx, userID, noteID)
		return err
	})
	return note, err
}

// explainFailedUpdate выясняет, почему UPDATE не затронул ни одной строки.
func (s *Store) explainFailedUpdate(ctx context.Context, tx pgx.Tx, userID, noteID string) error {
	var canEdit bool
	err := tx.QueryRow(ctx, `
		SELECT `+noteCanEdit+` FROM notes n WHERE n.id = $2 AND `+noteCanRead,
		userID, noteID).Scan(&canEdit)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return ErrNotFound
	case err != nil:
		return err
	case !canEdit:
		return ErrForbidden
	default:
		return ErrConflict
	}
}

// DeleteNote: удалить заметку может только владелец.
func (s *Store) DeleteNote(ctx context.Context, userID, noteID string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM notes WHERE id = $1 AND owner_id = $2`, noteID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
