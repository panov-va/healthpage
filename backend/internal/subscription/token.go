// Package subscription — общие примитивы подписок: capability-токены отписки (этап 3.4/3.5).
//
// Токен отписки — stateless HMAC от subscriber_id (DESIGN §3.5, §9). Это решение вместо
// случайного токена с хэшем в БД: ссылка отписки должна попадать в КАЖДОЕ письмо, но воркер
// читает подписчика из БД (где по §9 хранился бы только хэш — plaintext восстановить нельзя).
// HMAC решает это без хранения: воркер вычисляет токен из subscriber_id + секрет при рендере,
// эндпоинт отписки проверяет подпись. Колонка subscribers.unsubscribe_token при этом не
// используется (вестигиальна) — см. флаг в MEMORY.
package subscription

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// UnsubscribeToken возвращает токен отписки вида "<subscriberID>.<base64url(HMAC)>".
func UnsubscribeToken(secret string, subscriberID uuid.UUID) string {
	id := subscriberID.String()
	return id + "." + sign(secret, id)
}

// ParseUnsubscribeToken проверяет подпись токена и возвращает subscriber_id. Подпись сверяется
// в постоянном времени; при несовпадении/искажении — ошибка.
func ParseUnsubscribeToken(secret, token string) (uuid.UUID, error) {
	idStr, sig, ok := strings.Cut(token, ".")
	if !ok {
		return uuid.Nil, fmt.Errorf("subscription: malformed unsubscribe token")
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("subscription: bad subscriber id in token: %w", err)
	}
	if !hmac.Equal([]byte(sig), []byte(sign(secret, idStr))) {
		return uuid.Nil, fmt.Errorf("subscription: invalid unsubscribe token signature")
	}
	return id, nil
}

func sign(secret, msg string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msg))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
