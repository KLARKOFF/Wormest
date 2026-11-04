package api

import (
	"encoding/json"
	"errors"
	"net/http"
)

const maxJSONBody = 2 << 20 // 2 МБ хватит на очень длинную заметку

type errorBody struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorBody{Error: msg})
}

// decodeJSON читает тело запроса в dst. Неизвестные поля — ошибка: так опечатки во фронтенде видны сразу.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "Слишком большой запрос")
		} else {
			writeError(w, http.StatusBadRequest, "Некорректный JSON: "+err.Error())
		}
		return false
	}
	return true
}
