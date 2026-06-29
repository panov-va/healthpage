package feed

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
)

func testPage() domain.StatusPage {
	return domain.StatusPage{ID: uuid.New(), Name: "Acme", Slug: "acme", DefaultLocale: "ru"}
}

func TestBuildRSS(t *testing.T) {
	page := testPage()
	t0 := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	incidents := []domain.Incident{{
		ID: uuid.New(), Title: "API down", CurrentStatus: domain.IncidentInvestigating, Impact: domain.ImpactMajor,
		StartedAt: t0, Updates: []domain.IncidentUpdate{
			{Body: "looking", CreatedAt: t0},
			{Body: "fixed soon", CreatedAt: t0.Add(2 * time.Hour)},
		},
	}}
	maintenances := []domain.Maintenance{{
		ID: uuid.New(), Title: "DB upgrade", Status: domain.MaintenanceScheduled,
		ScheduledStart: t0.Add(48 * time.Hour), ScheduledEnd: t0.Add(50 * time.Hour),
	}}

	out, err := BuildRSS(page, incidents, maintenances, "https://h/")
	if err != nil {
		t.Fatalf("BuildRSS: %v", err)
	}
	if !strings.HasPrefix(string(out), "<?xml") {
		t.Error("нет XML-заголовка")
	}

	var parsed rss
	if err := xml.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("RSS не парсится: %v", err)
	}
	if parsed.Version != "2.0" {
		t.Errorf("version = %q", parsed.Version)
	}
	if len(parsed.Channel.Items) != 2 {
		t.Fatalf("ожидалось 2 элемента, got %d", len(parsed.Channel.Items))
	}
	// Работы (через 48ч) новее инцидента → первыми.
	if !strings.Contains(parsed.Channel.Items[0].Title, "DB upgrade") {
		t.Errorf("порядок по дате нарушен: первый = %q", parsed.Channel.Items[0].Title)
	}
	// Описание инцидента — из последнего обновления.
	inc := parsed.Channel.Items[1]
	if !strings.Contains(inc.Description, "fixed soon") {
		t.Errorf("описание инцидента не из последнего апдейта: %q", inc.Description)
	}
	if !strings.Contains(inc.Link, "/status/acme/incidents/") {
		t.Errorf("ссылка инцидента неверна: %q", inc.Link)
	}
}

func TestBuildRSSEscaping(t *testing.T) {
	page := testPage()
	incidents := []domain.Incident{{
		ID: uuid.New(), Title: "A & B <crash>", CurrentStatus: domain.IncidentInvestigating, Impact: domain.ImpactMinor,
		StartedAt: time.Now().UTC(),
	}}
	out, err := BuildRSS(page, incidents, nil, "https://h")
	if err != nil {
		t.Fatalf("BuildRSS: %v", err)
	}
	// encoding/xml экранирует — сырых < и & в значении быть не должно, но валидный XML парсится.
	var parsed rss
	if err := xml.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("XML с спецсимволами не парсится: %v", err)
	}
	if !strings.Contains(parsed.Channel.Items[0].Title, "A & B <crash>") {
		t.Errorf("заголовок после round-trip = %q", parsed.Channel.Items[0].Title)
	}
}

func TestBuildICal(t *testing.T) {
	page := testPage()
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	start := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	maintenances := []domain.Maintenance{{
		ID: uuid.New(), Title: "Upgrade; phase 1, core", Description: "Backend\nrollout",
		Status: domain.MaintenanceScheduled, ScheduledStart: start, ScheduledEnd: start.Add(time.Hour),
	}}

	out := string(BuildICal(page, maintenances, "https://h", now))

	for _, want := range []string{
		"BEGIN:VCALENDAR", "VERSION:2.0", "BEGIN:VEVENT", "END:VEVENT", "END:VCALENDAR",
		"DTSTART:20260701T100000Z", "DTEND:20260701T110000Z", "DTSTAMP:20260601T000000Z",
		"STATUS:TENTATIVE",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("iCal не содержит %q:\n%s", want, out)
		}
	}
	// Экранирование спецсимволов в SUMMARY/DESCRIPTION.
	if !strings.Contains(out, `SUMMARY:Upgrade\; phase 1\, core`) {
		t.Errorf("SUMMARY не экранирован:\n%s", out)
	}
	if !strings.Contains(out, `DESCRIPTION:Backend\nrollout`) {
		t.Errorf("DESCRIPTION не экранирован:\n%s", out)
	}
	// Строки завершаются CRLF.
	if !strings.Contains(out, "BEGIN:VCALENDAR\r\n") {
		t.Error("нет CRLF в конце строк")
	}
}

func TestBuildICalFolding(t *testing.T) {
	page := testPage()
	long := strings.Repeat("x", 200)
	maintenances := []domain.Maintenance{{
		ID: uuid.New(), Title: long, Status: domain.MaintenanceInProgress,
		ScheduledStart: time.Now().UTC(), ScheduledEnd: time.Now().UTC().Add(time.Hour),
	}}
	out := string(BuildICal(page, maintenances, "https://h", time.Now()))
	// Длинная строка свёрнута: продолжения начинаются с пробела; ни одна физическая строка > 75.
	for _, line := range strings.Split(out, "\r\n") {
		if len(line) > 75 {
			t.Errorf("строка длиннее 75 октетов (нет фолдинга): %d", len(line))
		}
	}
}
