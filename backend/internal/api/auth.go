package api

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"wormest/internal/store"
)

// Хеш-заглушка: при входе с несуществующим email всё равно сравниваем пароль,
// чтобы по времени ответа нельзя было узнать, зарегистрирован ли адрес.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password"), bcrypt.DefaultCost)

type credentials struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName,omitempty"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.RegistrationOpen {
		writeError(w, http.StatusForbidden, "Регистрация на этом сервере закрыта")
		return
	}
	var in credentials
	if !decodeJSON(w, r, &in) {
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.DisplayName = strings.TrimSpace(in.DisplayName)

	if _, err := mail.ParseAddress(in.Email); err != nil {
		writeError(w, http.StatusBadRequest, "Проверьте email")
		return
	}
	if n := utf8.RuneCountInString(in.Password); n < 8 || len(in.Password) > 72 {
		// 72 байта — предел bcrypt
		writeError(w, http.StatusBadRequest, "Пароль должен быть от 8 до 72 символов")
		return
	}
	if in.DisplayName == "" {
		in.DisplayName = strings.Split(in.Email, "@")[0]
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	u, err := s.store.CreateUser(r.Context(), in.Email, in.DisplayName, string(hash))
	if errors.Is(err, store.ErrEmailTaken) {
		writeError(w, http.StatusConflict, "Этот email уже зарегистрирован")
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if err := s.startSession(w, r, u.ID); err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !decodeJSON(w, r, &in) {
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))

	u, hash, err := s.store.UserByEmail(r.Context(), in.Email)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.internalError(w, r, err)
		return
	}
	if errors.Is(err, store.ErrNotFound) {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(in.Password))
		writeError(w, http.StatusUnauthorized, "Неверный email или пароль")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil {
		writeError(w, http.StatusUnauthorized, "Неверный email или пароль")
		return
	}
	if err := s.startSession(w, r, u.ID); err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := s.store.DeleteSession(r.Context(), hashToken(c.Value)); err != nil {
			s.internalError(w, r, err)
			return
		}
	}
	s.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request, u store.User) {
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, userID string) error {
	token, err := newSessionToken()
	if err != nil {
		return err
	}
	expires := time.Now().Add(s.cfg.SessionTTL)
	if err := s.store.CreateSession(r.Context(), userID, hashToken(token), expires); err != nil {
		return err
	}
	s.setSessionCookie(w, token, expires)
	return nil
}
