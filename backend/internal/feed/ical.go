package feed

import (
	"strings"
	"time"

	"github.com/healthpage/backend/internal/domain"
)

// BuildICal собирает iCal (RFC 5545) фид плановых работ страницы: по VEVENT на работу с окном
// проведения. now — момент генерации (для DTSTAMP); передаётся снаружи ради тестируемости.
func BuildICal(page domain.StatusPage, maintenances []domain.Maintenance, baseURL string, now time.Time) []byte {
	var b strings.Builder
	writeLine(&b, "BEGIN:VCALENDAR")
	writeLine(&b, "VERSION:2.0")
	writeLine(&b, "PRODID:-//HealthPage//Maintenance//"+strings.ToUpper(localeLang(page.DefaultLocale)))
	writeLine(&b, "CALSCALE:GREGORIAN")
	writeLine(&b, "METHOD:PUBLISH")
	writeLine(&b, "X-WR-CALNAME:"+escapeText(page.Name+" — maintenance"))

	stamp := now.UTC().Format(icalTimeFormat)
	url := pageURL(baseURL, page.Slug) + "/maintenances"
	for _, m := range maintenances {
		writeLine(&b, "BEGIN:VEVENT")
		writeLine(&b, "UID:maintenance-"+m.ID.String()+"@healthpage")
		writeLine(&b, "DTSTAMP:"+stamp)
		writeLine(&b, "DTSTART:"+m.ScheduledStart.UTC().Format(icalTimeFormat))
		writeLine(&b, "DTEND:"+m.ScheduledEnd.UTC().Format(icalTimeFormat))
		writeLine(&b, "SUMMARY:"+escapeText(m.Title))
		if m.Description != "" {
			writeLine(&b, "DESCRIPTION:"+escapeText(m.Description))
		}
		writeLine(&b, "STATUS:"+icalStatus(m.Status))
		writeLine(&b, "URL:"+escapeText(url))
		writeLine(&b, "END:VEVENT")
	}

	writeLine(&b, "END:VCALENDAR")
	return []byte(b.String())
}

// icalTimeFormat — UTC basic format RFC 5545 (например, 20260701T100000Z).
const icalTimeFormat = "20060102T150405Z"

// icalStatus отображает статус работ на STATUS VEVENT: запланированные — TENTATIVE, иначе CONFIRMED.
func icalStatus(s domain.MaintenanceStatus) string {
	if s == domain.MaintenanceScheduled {
		return "TENTATIVE"
	}
	return "CONFIRMED"
}

// writeLine добавляет строку с CRLF и фолдингом длинных строк (RFC 5545 §3.1, лимит ~75 октетов).
func writeLine(b *strings.Builder, line string) {
	const limit = 75
	if len(line) <= limit {
		b.WriteString(line)
		b.WriteString("\r\n")
		return
	}
	b.WriteString(line[:limit])
	b.WriteString("\r\n")
	rest := line[limit:]
	for len(rest) > limit-1 { // продолжения начинаются с пробела → лимит на 1 меньше
		b.WriteString(" ")
		b.WriteString(rest[:limit-1])
		b.WriteString("\r\n")
		rest = rest[limit-1:]
	}
	b.WriteString(" ")
	b.WriteString(rest)
	b.WriteString("\r\n")
}

// escapeText экранирует спецсимволы текстовых значений iCal (RFC 5545 §3.3.11).
func escapeText(s string) string {
	r := strings.NewReplacer(
		"\\", "\\\\",
		";", "\\;",
		",", "\\,",
		"\n", "\\n",
		"\r", "",
	)
	return r.Replace(s)
}

func localeLang(locale string) string {
	if strings.HasPrefix(strings.ToLower(locale), "en") {
		return "EN"
	}
	return "RU"
}
