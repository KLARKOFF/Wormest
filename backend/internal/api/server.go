// Package api — HTTP-интерфейс: маршруты, проверка входа, хендлеры.
package api

import (
	"log/slog"
	"net/http"

	"wormest/internal/config"
	"wormest/internal/store"
)

type Server struct {
	cfg   config.Config
	store *store.Store
	log   *slog.Logger
}

func NewServer(cfg config.Config, st *store.Store, log *slog.Logger) *Server {
	return &Server{cfg: cfg, store: st, log: log}
}

// Routes описывает весь REST API. С Go 1.22 стандартный ServeMux понимает метод и {параметры}.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)

	mux.HandleFunc("POST /api/auth/register", s.handleRegister)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	mux.Handle("GET /api/auth/me", s.requireUser(s.handleMe))

	mux.Handle("GET /api/notes", s.requireUser(s.handleListNotes))
	mux.Handle("POST /api/notes", s.requireUser(s.handleCreateNote))
	mux.Handle("GET /api/notes/{id}", s.requireUser(s.handleGetNote))
	mux.Handle("PUT /api/notes/{id}", s.requireUser(s.handleUpdateNote))
	mux.Handle("DELETE /api/notes/{id}", s.requireUser(s.handleDeleteNote))

	mux.Handle("GET /api/tags", s.requireUser(s.handleListTags))

	return s.recoverPanics(s.logRequests(mux))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
