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
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// GenerateConfirmToken возвращает непрозрачный confirm-токен (уходит в письмо double opt-in) и
// его SHA-256 хэш в hex (хранится в БД, §9). Сам токен в БД не хранится.
func GenerateConfirmToken() (token, hash string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("subscription: read random: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, HashConfirmToken(token), nil
}

// HashConfirmToken возвращает hex SHA-256 от confirm-токена (для поиска/сравнения в БД).
func HashConfirmToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

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

// SlackStateTTL — срок годности OAuth-state Slack (защита от повторного использования старого
// state; пользователь обычно проходит OAuth за секунды).
const SlackStateTTL = time.Hour

// SignSlackState возвращает CSRF-state для Slack OAuth, привязанный к странице:
// "<pageID>.<issuedUnix>.<base64url(HMAC)>". issuedAt — время выпуска (unix-секунды).
// State несёт, на какую страницу оформляется подписка (callback страницы не знает) и
// подтверждает, что OAuth инициировали мы.
func SignSlackState(secret string, pageID uuid.UUID, issuedAt int64) string {
	msg := pageID.String() + "." + strconv.FormatInt(issuedAt, 10)
	return msg + "." + sign(secret, msg)
}

// ParseSlackState проверяет подпись и срок годности state и возвращает page_id. now — текущее
// время (для проверки TTL). Подпись сверяется в постоянном времени.
func ParseSlackState(secret, state string, now time.Time) (uuid.UUID, error) {
	idStr, rest, ok := strings.Cut(state, ".")
	if !ok {
		return uuid.Nil, fmt.Errorf("subscription: malformed slack state")
	}
	issuedStr, sig, ok := strings.Cut(rest, ".")
	if !ok {
		return uuid.Nil, fmt.Errorf("subscription: malformed slack state")
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("subscription: bad page id in slack state: %w", err)
	}
	msg := idStr + "." + issuedStr
	if !hmac.Equal([]byte(sig), []byte(sign(secret, msg))) {
		return uuid.Nil, fmt.Errorf("subscription: invalid slack state signature")
	}
	issued, err := strconv.ParseInt(issuedStr, 10, 64)
	if err != nil {
		return uuid.Nil, fmt.Errorf("subscription: bad issued time in slack state: %w", err)
	}
	if now.Sub(time.Unix(issued, 0)) > SlackStateTTL {
		return uuid.Nil, fmt.Errorf("subscription: slack state expired")
	}
	return id, nil
}

func sign(secret, msg string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msg))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
