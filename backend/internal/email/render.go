package email

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/notify"
)

// RenderInput — данные для сборки письма по событию уведомления.
type RenderInput struct {
	Event          domain.EventType
	Locale         string // "ru" | "en" (дефолт ru)
	PageName       string
	PageURL        string // публичная страница статуса
	UnsubscribeURL string // для уведомлений (не для confirm)
	ConfirmURL     string // только для subscriber_confirm
	AccessURL      string // только для access_link (magic-link 4.2.1)
	Incident       *notify.IncidentPayload
	Maintenance    *notify.MaintenancePayload
}

// Content — отрендеренное письмо без получателя (его проставляет воркер).
type Content struct {
	Subject  string
	TextBody string
	HTMLBody string
}

// Render собирает тему и тело письма (text + html) на нужной локали.
func Render(in RenderInput) (Content, error) {
	t := dict(in.Locale)
	switch in.Event {
	case domain.EventIncidentNew, domain.EventIncidentUpdate:
		if in.Incident == nil {
			return Content{}, fmt.Errorf("email: nil incident payload for %s", in.Event)
		}
		return renderIncident(t, in), nil
	case domain.EventMaintenanceScheduled, domain.EventMaintenanceStarted, domain.EventMaintenanceCompleted:
		if in.Maintenance == nil {
			return Content{}, fmt.Errorf("email: nil maintenance payload for %s", in.Event)
		}
		return renderMaintenance(t, in), nil
	case domain.EventSubscriberConfirm:
		return renderConfirm(t, in), nil
	case domain.EventAccessLink:
		return renderAccessLink(t, in), nil
	default:
		return Content{}, fmt.Errorf("email: неизвестный тип события %q", in.Event)
	}
}

func renderIncident(t translations, in RenderInput) Content {
	p := in.Incident
	heading := t.eventTitle[in.Event]
	subject := fmt.Sprintf("[%s] %s: %s", in.PageName, heading, p.Title)

	lines := []string{
		heading + ": " + p.Title,
		t.statusLabel + ": " + t.incidentStatus[p.Status],
		t.impactLabel + ": " + t.impact[p.Impact],
	}
	if p.Body != "" {
		lines = append(lines, "", p.Body)
	}
	return assemble(t, subject, heading, lines, in)
}

func renderMaintenance(t translations, in RenderInput) Content {
	p := in.Maintenance
	heading := t.eventTitle[in.Event]
	subject := fmt.Sprintf("[%s] %s: %s", in.PageName, heading, p.Title)

	lines := []string{
		heading + ": " + p.Title,
		t.windowLabel + ": " + fmtTime(p.ScheduledStart) + " — " + fmtTime(p.ScheduledEnd),
	}
	if p.Description != "" {
		lines = append(lines, "", p.Description)
	}
	return assemble(t, subject, heading, lines, in)
}

func renderConfirm(t translations, in RenderInput) Content {
	subject := fmt.Sprintf("[%s] %s", in.PageName, t.confirmSubject)
	lines := []string{t.confirmIntro(in.PageName), "", t.confirmCTA + ": " + in.ConfirmURL}

	text := strings.Join(lines, "\n")
	var h strings.Builder
	h.WriteString("<p>" + html.EscapeString(t.confirmIntro(in.PageName)) + "</p>")
	h.WriteString(`<p><a href="` + html.EscapeString(in.ConfirmURL) + `">` + html.EscapeString(t.confirmCTA) + "</a></p>")
	return Content{Subject: subject, TextBody: text, HTMLBody: htmlDoc(h.String())}
}

func renderAccessLink(t translations, in RenderInput) Content {
	subject := fmt.Sprintf("[%s] %s", in.PageName, t.accessSubject)
	lines := []string{t.accessIntro(in.PageName), "", t.accessCTA + ": " + in.AccessURL}

	text := strings.Join(lines, "\n")
	var h strings.Builder
	h.WriteString("<p>" + html.EscapeString(t.accessIntro(in.PageName)) + "</p>")
	h.WriteString(`<p><a href="` + html.EscapeString(in.AccessURL) + `">` + html.EscapeString(t.accessCTA) + "</a></p>")
	return Content{Subject: subject, TextBody: text, HTMLBody: htmlDoc(h.String())}
}

// assemble дособирает общий «подвал» письма (ссылка на страницу + отписка) в обоих форматах.
func assemble(t translations, subject, heading string, lines []string, in RenderInput) Content {
	textParts := append([]string{}, lines...)
	if in.PageURL != "" {
		textParts = append(textParts, "", t.viewPage+": "+in.PageURL)
	}
	if in.UnsubscribeURL != "" {
		textParts = append(textParts, "", t.unsubscribe+": "+in.UnsubscribeURL)
	}
	text := strings.Join(textParts, "\n")

	var h strings.Builder
	h.WriteString("<h2>" + html.EscapeString(heading) + "</h2>")
	for _, ln := range lines {
		if ln == "" {
			continue
		}
		h.WriteString("<p>" + html.EscapeString(ln) + "</p>")
	}
	if in.PageURL != "" {
		h.WriteString(`<p><a href="` + html.EscapeString(in.PageURL) + `">` + html.EscapeString(t.viewPage) + "</a></p>")
	}
	if in.UnsubscribeURL != "" {
		h.WriteString(`<p style="color:#888;font-size:12px"><a href="` + html.EscapeString(in.UnsubscribeURL) + `">` + html.EscapeString(t.unsubscribe) + "</a></p>")
	}
	return Content{Subject: subject, TextBody: text, HTMLBody: htmlDoc(h.String())}
}

func htmlDoc(body string) string {
	return `<!doctype html><html><body style="font-family:sans-serif">` + body + "</body></html>"
}

func fmtTime(t time.Time) string { return t.UTC().Format("2006-01-02 15:04 MST") }

// ── i18n (минимальный RU/EN) ──

type translations struct {
	eventTitle     map[domain.EventType]string
	incidentStatus map[string]string
	impact         map[string]string
	statusLabel    string
	impactLabel    string
	windowLabel    string
	viewPage       string
	unsubscribe    string
	confirmSubject string
	confirmCTA     string
	confirmIntro   func(page string) string
	accessSubject  string
	accessCTA      string
	accessIntro    func(page string) string
}

func dict(locale string) translations {
	if strings.HasPrefix(strings.ToLower(locale), "en") {
		return translations{
			eventTitle: map[domain.EventType]string{
				domain.EventIncidentNew:          "New incident",
				domain.EventIncidentUpdate:       "Incident update",
				domain.EventMaintenanceScheduled: "Scheduled maintenance",
				domain.EventMaintenanceStarted:   "Maintenance started",
				domain.EventMaintenanceCompleted: "Maintenance completed",
			},
			incidentStatus: map[string]string{
				"investigating": "Investigating", "identified": "Identified",
				"monitoring": "Monitoring", "resolved": "Resolved",
			},
			impact: map[string]string{
				"none": "None", "minor": "Minor", "major": "Major", "critical": "Critical",
			},
			statusLabel: "Status", impactLabel: "Impact", windowLabel: "Window",
			viewPage: "View status page", unsubscribe: "Unsubscribe",
			confirmSubject: "Confirm your subscription", confirmCTA: "Confirm subscription",
			confirmIntro: func(page string) string {
				return "Please confirm your subscription to status updates for " + page + "."
			},
			accessSubject: "Access link", accessCTA: "Open status page",
			accessIntro: func(page string) string {
				return "Use the link below to access the private status page “" + page + "”. The link expires in 1 hour."
			},
		}
	}
	return translations{
		eventTitle: map[domain.EventType]string{
			domain.EventIncidentNew:          "Новый инцидент",
			domain.EventIncidentUpdate:       "Обновление инцидента",
			domain.EventMaintenanceScheduled: "Запланированы работы",
			domain.EventMaintenanceStarted:   "Работы начались",
			domain.EventMaintenanceCompleted: "Работы завершены",
		},
		incidentStatus: map[string]string{
			"investigating": "Расследуем", "identified": "Причина найдена",
			"monitoring": "Наблюдаем", "resolved": "Устранён",
		},
		impact: map[string]string{
			"none": "Нет", "minor": "Незначительное", "major": "Серьёзное", "critical": "Критическое",
		},
		statusLabel: "Статус", impactLabel: "Влияние", windowLabel: "Окно",
		viewPage: "Открыть страницу статуса", unsubscribe: "Отписаться",
		confirmSubject: "Подтвердите подписку", confirmCTA: "Подтвердить подписку",
		confirmIntro: func(page string) string {
			return "Подтвердите подписку на обновления статуса «" + page + "»."
		},
		accessSubject: "Ссылка для доступа", accessCTA: "Открыть страницу статуса",
		accessIntro: func(page string) string {
			return "Перейдите по ссылке ниже для доступа к приватной странице статуса «" + page + "». Ссылка действует 1 час."
		},
	}
}
