package webhookout

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/notify"
)

// RenderInput — данные для сборки исходящего webhook-payload по событию уведомления.
type RenderInput struct {
	Event       domain.EventType
	Locale      string // "ru" | "en" (дефолт ru)
	PageName    string
	PageURL     string
	Incident    *notify.IncidentPayload
	Maintenance *notify.MaintenancePayload
}

// payload — тело исходящего webhook'а. Поле text — Mattermost/Slack-совместимое (incoming webhook
// рендерит его как сообщение); остальные поля — структурированные данные для произвольных консьюмеров.
type payload struct {
	Text        string                     `json:"text"`
	Event       string                     `json:"event"`
	StatusPage  string                     `json:"status_page"`
	URL         string                     `json:"url,omitempty"`
	Incident    *notify.IncidentPayload    `json:"incident,omitempty"`
	Maintenance *notify.MaintenancePayload `json:"maintenance,omitempty"`
}

// Render собирает JSON-payload исходящего webhook'а (text + структурированные поля).
func Render(in RenderInput) ([]byte, error) {
	t := dict(in.Locale)
	p := payload{Event: string(in.Event), StatusPage: in.PageName, URL: in.PageURL}

	switch in.Event {
	case domain.EventIncidentNew, domain.EventIncidentUpdate:
		if in.Incident == nil {
			return nil, fmt.Errorf("webhookout: nil incident payload for %s", in.Event)
		}
		p.Incident = in.Incident
		p.Text = incidentText(t, in)
	case domain.EventMaintenanceScheduled, domain.EventMaintenanceStarted, domain.EventMaintenanceCompleted:
		if in.Maintenance == nil {
			return nil, fmt.Errorf("webhookout: nil maintenance payload for %s", in.Event)
		}
		p.Maintenance = in.Maintenance
		p.Text = maintenanceText(t, in)
	default:
		return nil, fmt.Errorf("webhookout: неизвестный тип события %q", in.Event)
	}
	return json.Marshal(p)
}

func incidentText(t translations, in RenderInput) string {
	p := in.Incident
	var b strings.Builder
	fmt.Fprintf(&b, "**%s: %s**\n", t.eventTitle[in.Event], p.Title)
	fmt.Fprintf(&b, "%s: %s | %s: %s", t.statusLabel, t.incidentStatus[p.Status], t.impactLabel, t.impact[p.Impact])
	if p.Body != "" {
		fmt.Fprintf(&b, "\n%s", p.Body)
	}
	appendPageLink(&b, t, in.PageURL)
	return b.String()
}

func maintenanceText(t translations, in RenderInput) string {
	p := in.Maintenance
	var b strings.Builder
	fmt.Fprintf(&b, "**%s: %s**\n", t.eventTitle[in.Event], p.Title)
	fmt.Fprintf(&b, "%s: %s — %s", t.windowLabel, fmtTime(p.ScheduledStart), fmtTime(p.ScheduledEnd))
	if p.Description != "" {
		fmt.Fprintf(&b, "\n%s", p.Description)
	}
	appendPageLink(&b, t, in.PageURL)
	return b.String()
}

func appendPageLink(b *strings.Builder, t translations, url string) {
	if url != "" {
		fmt.Fprintf(b, "\n%s: %s", t.viewPage, url)
	}
}

func fmtTime(t time.Time) string { return t.UTC().Format("2006-01-02 15:04 MST") }

// ── i18n (минимальный RU/EN, симметрично slack/telegram/email) ──

type translations struct {
	eventTitle     map[domain.EventType]string
	incidentStatus map[string]string
	impact         map[string]string
	statusLabel    string
	impactLabel    string
	windowLabel    string
	viewPage       string
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
			viewPage: "Status page",
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
		viewPage: "Страница статуса",
	}
}
