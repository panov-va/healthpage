package telegram

import (
	"strings"
	"testing"
	"time"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/notify"
)

func TestRenderIncidentRU(t *testing.T) {
	out, err := Render(RenderInput{
		Event:    domain.EventIncidentNew,
		Locale:   "ru",
		PageName: "Acme",
		PageURL:  "https://h/status/acme",
		Incident: &notify.IncidentPayload{
			Title: "Сбой API", Status: "investigating", Impact: "major", Body: "Чиним",
		},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, want := range []string{
		"<b>Новый инцидент: Сбой API</b>", "Статус: Расследуем", "Влияние: Серьёзное",
		"Чиним", "https://h/status/acme", "Открыть страницу статуса", "/stop",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("в сообщении нет %q:\n%s", want, out)
		}
	}
}

func TestRenderMaintenanceEN(t *testing.T) {
	start := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	out, err := Render(RenderInput{
		Event:    domain.EventMaintenanceScheduled,
		Locale:   "en",
		PageName: "Acme",
		Maintenance: &notify.MaintenancePayload{
			Title: "DB upgrade", Status: "scheduled", ScheduledStart: start, ScheduledEnd: start.Add(time.Hour),
		},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, want := range []string{
		"<b>Scheduled maintenance: DB upgrade</b>", "Window:", "2026-07-01 10:00 UTC", "To unsubscribe",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

func TestRenderEscapesHTML(t *testing.T) {
	out, err := Render(RenderInput{
		Event:    domain.EventIncidentNew,
		Locale:   "ru",
		PageName: "Acme",
		Incident: &notify.IncidentPayload{Title: "<b>x</b> & y", Status: "investigating", Impact: "none"},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(out, "<b>x</b>") {
		t.Errorf("пользовательский HTML не экранирован:\n%s", out)
	}
	if !strings.Contains(out, "&lt;b&gt;x&lt;/b&gt; &amp; y") {
		t.Errorf("ожидалось экранирование заголовка:\n%s", out)
	}
}

func TestRenderErrors(t *testing.T) {
	if _, err := Render(RenderInput{Event: domain.EventIncidentNew}); err == nil {
		t.Error("nil incident payload должен дать ошибку")
	}
	if _, err := Render(RenderInput{Event: domain.EventMaintenanceScheduled}); err == nil {
		t.Error("nil maintenance payload должен дать ошибку")
	}
	if _, err := Render(RenderInput{Event: domain.EventSubscriberConfirm}); err == nil {
		t.Error("неподдерживаемое событие должно дать ошибку")
	}
}
