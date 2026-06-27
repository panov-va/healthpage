package api

import (
	"encoding/json"
	"log"
	"net/http"
)

// errorBody — формат ошибки по контракту: {"error":{"code","message"}} (openapi Error).
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeJSON сериализует v и пишет ответ с заданным статусом.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("api: encode response: %v", err)
	}
}

// writeError пишет ошибку в формате контракта.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message}})
}

// decodeJSON читает тело запроса в dst с ограничением размера. Возвращает false и пишет
// 400, если тело невалидно.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "невалидный JSON в теле запроса")
		return false
	}
	return true
}

// decodeBodyQuiet читает тело в dst, не записывая ответ при ошибке (для опциональных тел,
// напр. refresh-токен, который обычно приходит в cookie).
func decodeBodyQuiet(r *http.Request, dst any) error {
	return json.NewDecoder(r.Body).Decode(dst)
}
