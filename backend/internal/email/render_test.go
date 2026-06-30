package email

import (
	"strings"
	"testing"
	"time"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/notify"
)

func TestRenderIncidentRU(t *testing.T) {
	c, err := Render(RenderInput{
		Event:          domain.EventIncidentNew,
		Locale:         "ru",
		PageName:       "Acme",
		PageURL:        "https://h/status/acme",
		UnsubscribeURL: "https://h/api/v1/unsubscribe?token=t",
		Incident:       &notify.IncidentPayload{Title: "API лежит", Status: "investigating", Impact: "major", Body: "Чиним"},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(c.Subject, "Acme") || !strings.Contains(c.Subject, "API лежит") {
		t.Errorf("subject не содержит имя страницы/заголовок: %q", c.Subject)
	}
	for _, want := range []string{"Новый инцидент", "Расследуем", "Серьёзное", "Чиним", "Отписаться"} {
		if !strings.Contains(c.TextBody, want) {
			t.Errorf("text не содержит %q: %s", want, c.TextBody)
		}
	}
	if !strings.Contains(c.HTMLBody, "unsubscribe?token=t") {
		t.Errorf("html не содержит ссылку отписки: %s", c.HTMLBody)
	}
}

func TestRenderMaintenanceEN(t *testing.T) {
	start := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	c, err := Render(RenderInput{
		Event:       domain.EventMaintenanceStarted,
		Locale:      "en",
		PageName:    "Acme",
		Maintenance: &notify.MaintenancePayload{Title: "DB upgrade", Status: "in_progress", ScheduledStart: start, ScheduledEnd: start.Add(time.Hour)},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(c.Subject, "Maintenance started") || !strings.Contains(c.TextBody, "DB upgrade") {
		t.Errorf("subject/text неверны: %q / %s", c.Subject, c.TextBody)
	}
	if !strings.Contains(c.TextBody, "2026-07-01 10:00") {
		t.Errorf("text не содержит окно работ: %s", c.TextBody)
	}
}

func TestRenderConfirm(t *testing.T) {
	c, err := Render(RenderInput{
		Event:      domain.EventSubscriberConfirm,
		Locale:     "ru",
		PageName:   "Acme",
		ConfirmURL: "https://h/api/v1/subscribe/confirm?token=abc",
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(c.Subject, "Подтвердите подписку") {
		t.Errorf("subject: %q", c.Subject)
	}
	if !strings.Contains(c.HTMLBody, "confirm?token=abc") {
		t.Errorf("html не содержит ссылку подтверждения: %s", c.HTMLBody)
	}
	// В письме подтверждения не должно быть ссылки отписки.
	if strings.Contains(c.TextBody, "Отписаться") {
		t.Errorf("в confirm-письме не должно быть отписки: %s", c.TextBody)
	}
}

func TestRenderAccessLink(t *testing.T) {
	c, err := Render(RenderInput{
		Event:     domain.EventAccessLink,
		Locale:    "en",
		PageName:  "Acme",
		AccessURL: "https://h/status/acme/access/verify?token=xyz",
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(c.Subject, "Access link") {
		t.Errorf("subject: %q", c.Subject)
	}
	if !strings.Contains(c.HTMLBody, "access/verify?token=xyz") {
		t.Errorf("html не содержит magic-link: %s", c.HTMLBody)
	}
}

func TestRenderErrors(t *testing.T) {
	if _, err := Render(RenderInput{Event: domain.EventIncidentNew}); err == nil {
		t.Error("ожидалась ошибка при nil incident payload")
	}
	if _, err := Render(RenderInput{Event: "bogus"}); err == nil {
		t.Error("ожидалась ошибка при неизвестном событии")
	}
}
