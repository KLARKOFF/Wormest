package store

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
)

type TagCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func (s *Store) ListTags(ctx context.Context, userID string) ([]TagCount, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT t.name, count(nt.note_id)::int
		FROM tags t LEFT JOIN note_tags nt ON nt.tag_id = t.id
		WHERE t.owner_id = $1
		GROUP BY t.name
		ORDER BY t.name`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (TagCount, error) {
		var t TagCount
		err := r.Scan(&t.Name, &t.Count)
		return t, err
	})
}

// NormalizeTags приводит теги к единому виду: без «#», в нижнем регистре, без пустых и повторов.
func NormalizeTags(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, t := range in {
		t = strings.ToLower(strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(t), "#")))
		if t == "" || len([]rune(t)) > 64 || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// setNoteTags полностью заменяет теги заметки. Теги принадлежат владельцу заметки,
// даже если редактирует другой пользователь с доступом.
func setNoteTags(ctx context.Context, tx pgx.Tx, noteID, ownerID string, names []string) error {
	names = NormalizeTags(names)
	if _, err := tx.Exec(ctx, `DELETE FROM note_tags WHERE note_id = $1`, noteID); err != nil {
		return err
	}
	if len(names) > 0 {
		if _, err := tx.Exec(ctx, `
			INSERT INTO tags (owner_id, name) SELECT $1, unnest($2::text[])
			ON CONFLICT (owner_id, name) DO NOTHING`, ownerID, names); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO note_tags (note_id, tag_id)
			SELECT $1, id FROM tags WHERE owner_id = $2 AND name = ANY($3::text[])`,
			noteID, ownerID, names); err != nil {
			return err
		}
	}
	// Убираем теги, которые больше нигде не используются
	_, err := tx.Exec(ctx, `
		DELETE FROM tags t WHERE t.owner_id = $1
		  AND NOT EXISTS (SELECT 1 FROM note_tags WHERE tag_id = t.id)
		  AND NOT EXISTS (SELECT 1 FROM document_tags WHERE tag_id = t.id)`, ownerID)
	return err
}
