package subscription

import (
	"testing"

	"github.com/google/uuid"
)

func TestUnsubscribeTokenRoundTrip(t *testing.T) {
	const secret = "test-secret"
	id := uuid.New()

	tok := UnsubscribeToken(secret, id)
	got, err := ParseUnsubscribeToken(secret, tok)
	if err != nil {
		t.Fatalf("ParseUnsubscribeToken: %v", err)
	}
	if got != id {
		t.Errorf("round-trip id = %s, want %s", got, id)
	}
}

func TestUnsubscribeTokenRejectsTampering(t *testing.T) {
	const secret = "test-secret"
	id := uuid.New()
	tok := UnsubscribeToken(secret, id)

	// Чужой секрет — подпись не сойдётся.
	if _, err := ParseUnsubscribeToken("other-secret", tok); err == nil {
		t.Error("ожидалась ошибка при другом секрете")
	}
	// Подмена subscriber_id при сохранённой подписи.
	other := uuid.New().String() + tok[len(uuid.Nil.String()):]
	if _, err := ParseUnsubscribeToken(secret, other); err == nil {
		t.Error("ожидалась ошибка при подмене id")
	}
	// Совсем кривой формат.
	if _, err := ParseUnsubscribeToken(secret, "garbage"); err == nil {
		t.Error("ожидалась ошибка при отсутствии разделителя")
	}
}
