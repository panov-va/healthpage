package telegram

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/notify"
)

// RenderInput — данные для сборки сообщения бота по событию уведомления.
type RenderInput struct {
	Event       domain.EventType
	Locale      string // "ru" | "en" (дефолт ru)
	PageName    string
	PageURL     string // публичная страница статуса
	Incident    *notify.IncidentPayload
	Maintenance *notify.MaintenancePayload
}

// Render собирает текст сообщения (parse_mode=HTML) на нужной локали.
func Render(in RenderInput) (string, error) {
	t := dict(in.Locale)
	switch in.Event {
	case domain.EventIncidentNew, domain.EventIncidentUpdate:
		if in.Incident == nil {
			return "", fmt.Errorf("telegram: nil incident payload for %s", in.Event)
		}
		return renderIncident(t, in), nil
	case domain.EventMaintenanceScheduled, domain.EventMaintenanceStarted, domain.EventMaintenanceCompleted:
		if in.Maintenance == nil {
			return "", fmt.Errorf("telegram: nil maintenance payload for %s", in.Event)
		}
		return renderMaintenance(t, in), nil
	default:
		return "", fmt.Errorf("telegram: неизвестный тип события %q", in.Event)
	}
}

func renderIncident(t translations, in RenderInput) string {
	p := in.Incident
	heading := t.eventTitle[in.Event]
	lines := []string{
		"<b>" + esc(heading+": "+p.Title) + "</b>",
		esc(t.statusLabel + ": " + t.incidentStatus[p.Status]),
		esc(t.impactLabel + ": " + t.impact[p.Impact]),
	}
	if p.Body != "" {
		lines = append(lines, "", esc(p.Body))
	}
	return assemble(t, lines, in)
}

func renderMaintenance(t translations, in RenderInput) string {
	p := in.Maintenance
	heading := t.eventTitle[in.Event]
	lines := []string{
		"<b>" + esc(heading+": "+p.Title) + "</b>",
		esc(t.windowLabel + ": " + fmtTime(p.ScheduledStart) + " — " + fmtTime(p.ScheduledEnd)),
	}
	if p.Description != "" {
		lines = append(lines, "", esc(p.Description))
	}
	return assemble(t, lines, in)
}

// assemble дособирает «подвал» сообщения (ссылка на страницу + подсказка про отписку).
func assemble(t translations, lines []string, in RenderInput) string {
	if in.PageURL != "" {
		lines = append(lines, "", `<a href="`+esc(in.PageURL)+`">`+esc(t.viewPage)+"</a>")
	}
	lines = append(lines, "", esc(t.unsubscribeHint))
	return strings.Join(lines, "\n")
}

// esc экранирует текст для parse_mode=HTML Telegram (минимум: & < > ).
func esc(s string) string { return html.EscapeString(s) }

func fmtTime(t time.Time) string { return t.UTC().Format("2006-01-02 15:04 MST") }

// ── i18n (минимальный RU/EN, симметрично email) ──

type translations struct {
	eventTitle      map[domain.EventType]string
	incidentStatus  map[string]string
	impact          map[string]string
	statusLabel     string
	impactLabel     string
	windowLabel     string
	viewPage        string
	unsubscribeHint string
	// тексты команд бота
	startNoArg    string
	pageNotFound  string
	subscribed    func(page string) string
	already       func(page string) string
	stoppedOne    func(page string) string
	stoppedAll    func(n int) string
	notSubscribed string
	help          string
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
			viewPage:        "View status page",
			unsubscribeHint: "To unsubscribe, send /stop",
			startNoArg:      "Open this bot via the subscription link on a status page to subscribe.",
			pageNotFound:    "Status page not found. Use the subscription link on the page.",
			subscribed:      func(page string) string { return "Subscribed to status updates for " + page + "." },
			already:         func(page string) string { return "You are already subscribed to " + page + "." },
			stoppedOne:      func(page string) string { return "Unsubscribed from " + page + "." },
			stoppedAll:      func(n int) string { return fmt.Sprintf("Unsubscribed from %d page(s).", n) },
			notSubscribed:   "You have no active subscriptions.",
			help:            "Commands:\n/start <page> — subscribe (use the page's link)\n/stop — unsubscribe from all\n/stop <page> — unsubscribe from one page",
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
		viewPage:        "Открыть страницу статуса",
		unsubscribeHint: "Чтобы отписаться, отправьте /stop",
		startNoArg:      "Откройте этого бота по ссылке подписки на странице статуса, чтобы подписаться.",
		pageNotFound:    "Страница статуса не найдена. Используйте ссылку подписки на самой странице.",
		subscribed: func(page string) string {
			return "Вы подписаны на обновления статуса «" + page + "»."
		},
		already:       func(page string) string { return "Вы уже подписаны на «" + page + "»." },
		stoppedOne:    func(page string) string { return "Отписка от «" + page + "» выполнена." },
		stoppedAll:    func(n int) string { return fmt.Sprintf("Отписка выполнена (страниц: %d).", n) },
		notSubscribed: "У вас нет активных подписок.",
		help:          "Команды:\n/start <страница> — подписаться (используйте ссылку со страницы)\n/stop — отписаться от всего\n/stop <страница> — отписаться от одной страницы",
	}
}
