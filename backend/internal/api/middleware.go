package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"wormest/internal/store"
)

const sessionCookie = "wormest_session"

// userHandler — хендлер, которому уже известен вошедший пользователь.
type userHandler func(w http.ResponseWriter, r *http.Request, u store.User)

// requireUser пропускает запрос дальше, только если в куке лежит действующая сессия.
func (s *Server) requireUser(next userHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "Нужно войти")
			return
		}
		u, err := s.store.UserBySession(r.Context(), hashToken(c.Value))
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "Сессия истекла, войдите снова")
			return
		}
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		next(w, r, u)
	})
}

func newSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

// SameSite=Lax: браузер не отправит куку в POST/PUT/DELETE с чужого сайта — это защита от CSRF.
func (s *Server) setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.log.Info("запрос", "method", r.Method, "path", r.URL.Path,
			"status", rec.status, "ms", time.Since(start).Milliseconds())
	})
}

func (s *Server) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				s.log.Error("паника", "value", v, "path", r.URL.Path)
				writeError(w, http.StatusInternalServerError, "Внутренняя ошибка сервера")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) internalError(w http.ResponseWriter, r *http.Request, err error) {
	s.log.Error("ошибка", "err", err, "method", r.Method, "path", r.URL.Path)
	writeError(w, http.StatusInternalServerError, "Внутренняя ошибка сервера")
}
