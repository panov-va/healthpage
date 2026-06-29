package slack

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/notify"
)

// RenderInput — данные для сборки Slack-сообщения по событию уведомления.
type RenderInput struct {
	Event       domain.EventType
	Locale      string // "ru" | "en" (дефолт ru)
	PageName    string
	PageURL     string // публичная страница статуса
	Incident    *notify.IncidentPayload
	Maintenance *notify.MaintenancePayload
}

// Render собирает JSON-payload сообщения (Slack Block Kit, через attachment с цветом по impact).
func Render(in RenderInput) ([]byte, error) {
	t := dict(in.Locale)
	var color string
	var blocks []map[string]any

	switch in.Event {
	case domain.EventIncidentNew, domain.EventIncidentUpdate:
		if in.Incident == nil {
			return nil, fmt.Errorf("slack: nil incident payload for %s", in.Event)
		}
		color = impactColor[in.Incident.Impact]
		blocks = incidentBlocks(t, in)
	case domain.EventMaintenanceScheduled, domain.EventMaintenanceStarted, domain.EventMaintenanceCompleted:
		if in.Maintenance == nil {
			return nil, fmt.Errorf("slack: nil maintenance payload for %s", in.Event)
		}
		color = maintenanceColor
		blocks = maintenanceBlocks(t, in)
	default:
		return nil, fmt.Errorf("slack: неизвестный тип события %q", in.Event)
	}

	payload := map[string]any{
		"attachments": []map[string]any{
			{"color": color, "blocks": blocks},
		},
	}
	return json.Marshal(payload)
}

func incidentBlocks(t translations, in RenderInput) []map[string]any {
	p := in.Incident
	heading := t.eventTitle[in.Event]
	blocks := []map[string]any{
		headerBlock(heading + ": " + p.Title),
		{"type": "section", "fields": []map[string]any{
			mrkdwn("*" + t.statusLabel + ":*\n" + t.incidentStatus[p.Status]),
			mrkdwn("*" + t.impactLabel + ":*\n" + t.impact[p.Impact]),
		}},
	}
	if p.Body != "" {
		blocks = append(blocks, map[string]any{"type": "section", "text": mrkdwn(escMrkdwn(p.Body))})
	}
	return append(blocks, footerBlocks(t, in)...)
}

func maintenanceBlocks(t translations, in RenderInput) []map[string]any {
	p := in.Maintenance
	heading := t.eventTitle[in.Event]
	blocks := []map[string]any{
		headerBlock(heading + ": " + p.Title),
		{"type": "section", "text": mrkdwn("*" + t.windowLabel + ":*\n" + fmtTime(p.ScheduledStart) + " — " + fmtTime(p.ScheduledEnd))},
	}
	if p.Description != "" {
		blocks = append(blocks, map[string]any{"type": "section", "text": mrkdwn(escMrkdwn(p.Description))})
	}
	return append(blocks, footerBlocks(t, in)...)
}

// footerBlocks — context-блок со ссылкой на страницу статуса.
func footerBlocks(t translations, in RenderInput) []map[string]any {
	if in.PageURL == "" {
		return nil
	}
	link := "<" + in.PageURL + "|" + escMrkdwn(t.viewPage) + ">"
	return []map[string]any{
		{"type": "context", "elements": []map[string]any{mrkdwn(link)}},
	}
}

func headerBlock(text string) map[string]any {
	return map[string]any{"type": "header", "text": map[string]any{
		"type": "plain_text", "text": truncate(text, 150), "emoji": true,
	}}
}

func mrkdwn(text string) map[string]any {
	return map[string]any{"type": "mrkdwn", "text": text}
}

// escMrkdwn экранирует спецсимволы Slack mrkdwn (& < >). Применяется к пользовательскому тексту.
func escMrkdwn(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func fmtTime(t time.Time) string { return t.UTC().Format("2006-01-02 15:04 MST") }

// Цвета вложения по impact (полоса слева от сообщения) и для работ.
var impactColor = map[string]string{
	"none": "#36a64f", "minor": "#e0b400", "major": "#e8730c", "critical": "#d40e0e",
}

const maintenanceColor = "#2f6fdb"

// ── i18n (минимальный RU/EN, симметрично email/telegram) ──

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
			viewPage: "View status page",
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
		viewPage: "Открыть страницу статуса",
	}
}
