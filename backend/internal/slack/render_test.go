package slack

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/notify"
)

// decode разбирает payload в удобную для проверок структуру.
func decode(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("payload не JSON: %v", err)
	}
	return m
}

func TestRenderIncidentRU(t *testing.T) {
	raw, err := Render(RenderInput{
		Event:    domain.EventIncidentNew,
		Locale:   "ru",
		PageName: "Acme",
		PageURL:  "https://h/status/acme",
		Incident: &notify.IncidentPayload{Title: "Сбой API", Status: "investigating", Impact: "critical", Body: "Чиним"},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	m := decode(t, raw)
	atts, ok := m["attachments"].([]any)
	if !ok || len(atts) != 1 {
		t.Fatalf("ожидался один attachment: %v", m)
	}
	att := atts[0].(map[string]any)
	if att["color"] != impactColor["critical"] {
		t.Errorf("цвет по impact неверен: %v", att["color"])
	}
	s := string(raw)
	for _, want := range []string{"Новый инцидент: Сбой API", "Расследуем", "Критическое", "Чиним", "https://h/status/acme", "Открыть страницу статуса"} {
		if !strings.Contains(s, want) {
			t.Errorf("в payload нет %q:\n%s", want, s)
		}
	}
}

func TestRenderMaintenanceEN(t *testing.T) {
	start := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	raw, err := Render(RenderInput{
		Event:       domain.EventMaintenanceScheduled,
		Locale:      "en",
		PageName:    "Acme",
		Maintenance: &notify.MaintenancePayload{Title: "DB upgrade", Status: "scheduled", ScheduledStart: start, ScheduledEnd: start.Add(time.Hour)},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(raw)
	for _, want := range []string{"Scheduled maintenance: DB upgrade", "Window", "2026-07-01 10:00 UTC"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q:\n%s", want, s)
		}
	}
	m := decode(t, raw)
	att := m["attachments"].([]any)[0].(map[string]any)
	if att["color"] != maintenanceColor {
		t.Errorf("цвет работ неверен: %v", att["color"])
	}
}

func TestRenderEscapesMrkdwn(t *testing.T) {
	raw, err := Render(RenderInput{
		Event:    domain.EventIncidentNew,
		Locale:   "ru",
		PageName: "Acme",
		Incident: &notify.IncidentPayload{Title: "x", Status: "investigating", Impact: "none", Body: "a < b & c > d"},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	// Проверяем ДЕКОДИРОВАННЫЙ payload: encoding/json дополнительно экранирует &<> в & и т.п.
	// в байтах, но Slack их раскодирует обратно — значимо именно содержимое строки.
	m := decode(t, raw)
	blocks := m["attachments"].([]any)[0].(map[string]any)["blocks"].([]any)
	var texts []string
	for _, b := range blocks {
		if txt, ok := b.(map[string]any)["text"].(map[string]any); ok {
			texts = append(texts, txt["text"].(string))
		}
	}
	joined := strings.Join(texts, "\n")
	if !strings.Contains(joined, "a &lt; b &amp; c &gt; d") {
		t.Errorf("mrkdwn не экранирован:\n%s", joined)
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
