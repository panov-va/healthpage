// Package webhook содержит чистую логику входящих webhook-интеграций (этап 5.3):
// проверку HMAC-подписи, парсинг payload'ов Grafana/Prometheus в нормализованные алерты и
// маппинг алертов на компоненты. Без БД/HTTP — переиспользуется хендлерами и тестами.
package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// VerifySignature проверяет HMAC-SHA256-подпись тела секретом интеграции.
// header — значение X-Signature; допускается префикс "sha256=" (как у GitHub/Grafana).
// Сравнение — constant-time. Пустой секрет/подпись → false.
func VerifySignature(secret string, body []byte, header string) bool {
	if secret == "" || header == "" {
		return false
	}
	got := strings.TrimSpace(header)
	got = strings.TrimPrefix(got, "sha256=")

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(strings.ToLower(got)), []byte(want))
}

// Sign возвращает hex HMAC-SHA256 тела секретом (для тестов и документации интеграции).
func Sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
