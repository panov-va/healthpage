package subscription

import (
	"testing"
	"time"

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

func TestSlackStateRoundTrip(t *testing.T) {
	const secret = "test-secret"
	pageID := uuid.New()
	now := time.Unix(1_700_000_000, 0)

	state := SignSlackState(secret, pageID, now.Unix())
	got, err := ParseSlackState(secret, state, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("ParseSlackState: %v", err)
	}
	if got != pageID {
		t.Errorf("round-trip page id = %s, want %s", got, pageID)
	}
}

func TestSlackStateRejects(t *testing.T) {
	const secret = "test-secret"
	pageID := uuid.New()
	now := time.Unix(1_700_000_000, 0)
	state := SignSlackState(secret, pageID, now.Unix())

	// Чужой секрет.
	if _, err := ParseSlackState("other", state, now); err == nil {
		t.Error("ожидалась ошибка при другом секрете")
	}
	// Истёкший state.
	if _, err := ParseSlackState(secret, state, now.Add(SlackStateTTL+time.Minute)); err == nil {
		t.Error("ожидалась ошибка при истёкшем state")
	}
	// Кривой формат.
	if _, err := ParseSlackState(secret, "garbage", now); err == nil {
		t.Error("ожидалась ошибка при кривом формате")
	}
}
