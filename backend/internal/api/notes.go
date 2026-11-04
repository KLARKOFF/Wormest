package api

import (
	"errors"
	"net/http"
	"strings"

	"wormest/internal/store"
)

func (s *Server) handleListNotes(w http.ResponseWriter, r *http.Request, u store.User) {
	tag := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("tag")))
	notes, err := s.store.ListNotes(r.Context(), u.ID, tag)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, notes)
}

func (s *Server) handleGetNote(w http.ResponseWriter, r *http.Request, u store.User) {
	id, ok := noteID(w, r)
	if !ok {
		return
	}
	note, err := s.store.GetNote(r.Context(), u.ID, id)
	if err != nil {
		s.noteError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, note)
}

func (s *Server) handleCreateNote(w http.ResponseWriter, r *http.Request, u store.User) {
	var in store.NoteInput
	if !decodeJSON(w, r, &in) || !validNoteInput(w, &in) {
		return
	}
	note, err := s.store.CreateNote(r.Context(), u.ID, in)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, note)
}

type updateNoteRequest struct {
	store.NoteInput
	// Версия, которую видел клиент перед правкой
	Version int `json:"version"`
}

func (s *Server) handleUpdateNote(w http.ResponseWriter, r *http.Request, u store.User) {
	id, ok := noteID(w, r)
	if !ok {
		return
	}
	var in updateNoteRequest
	if !decodeJSON(w, r, &in) || !validNoteInput(w, &in.NoteInput) {
		return
	}
	note, err := s.store.UpdateNote(r.Context(), u.ID, id, in.Version, in.NoteInput)
	if errors.Is(err, store.ErrConflict) {
		// Отдаём актуальную версию, чтобы клиент мог предложить «загрузить» или «перезаписать»
		current, getErr := s.store.GetNote(r.Context(), u.ID, id)
		if getErr != nil {
			s.noteError(w, r, getErr)
			return
		}
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": "Заметку изменили на другом устройстве",
			"note":  current,
		})
		return
	}
	if err != nil {
		s.noteError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, note)
}

func (s *Server) handleDeleteNote(w http.ResponseWriter, r *http.Request, u store.User) {
	id, ok := noteID(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteNote(r.Context(), u.ID, id); err != nil {
		s.noteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListTags(w http.ResponseWriter, r *http.Request, u store.User) {
	tags, err := s.store.ListTags(r.Context(), u.ID)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, tags)
}

func noteID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !store.ValidID(id) {
		writeError(w, http.StatusNotFound, "Заметка не найдена")
		return "", false
	}
	return id, true
}

func validNoteInput(w http.ResponseWriter, in *store.NoteInput) bool {
	in.Title = strings.TrimSpace(in.Title)
	if len([]rune(in.Title)) > 300 {
		writeError(w, http.StatusBadRequest, "Заголовок длиннее 300 символов")
		return false
	}
	if in.DocumentID != nil && !store.ValidID(*in.DocumentID) {
		writeError(w, http.StatusBadRequest, "Некорректный documentId")
		return false
	}
	if in.Tags == nil {
		in.Tags = []string{}
	}
	return true
}

func (s *Server) noteError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "Заметка не найдена")
	case errors.Is(err, store.ErrForbidden):
		writeError(w, http.StatusForbidden, "Нет прав на изменение этой заметки")
	default:
		s.internalError(w, r, err)
	}
}
