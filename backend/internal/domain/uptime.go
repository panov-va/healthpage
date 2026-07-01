package domain

import (
	"time"

	"github.com/google/uuid"
)

// ── Расчёт uptime (DESIGN §6, §7.1; этап 7.1) ──
//
// Uptime — доля доступного времени компонента за период (по ComponentStatusHistory).
// Правила (DESIGN §6, [РЕШЕНО]):
//   - under_maintenance НЕ считается downtime — время плановых работ ИСКЛЮЧАЕТСЯ из расчёта
//     (не в числитель, не в знаменатель), как в Statuspage.
//   - downtime = время в partial_outage и major_outage.
//   - operational и degraded_performance считаются доступными (degraded — сервис доступен, но
//     деградирован; для MVP не понижает uptime).
//   - до создания компонента / до первой записи истории время считается доступным (100%).

// DailyUptime — доступность за один календарный день (UTC).
type DailyUptime struct {
	Date    time.Time // начало дня (UTC, усечено до суток)
	Percent float64
}

// UptimeReport — отчёт доступности компонента за период.
type UptimeReport struct {
	ComponentID   uuid.UUID
	Days          int
	UptimePercent float64
	Daily         []DailyUptime
}

// statusIsDown сообщает, что статус считается недоступностью (downtime).
func statusIsDown(s ComponentStatus) bool {
	return s == StatusPartialOutage || s == StatusMajorOutage
}

// ComputeUptime строит отчёт доступности за `days` календарных дней, заканчивающихся now.
// periods — история статусов компонента (любого объёма; будет клиппирована к окну).
// createdAt ограничивает окно снизу (до создания компонент недоступности не имел).
func ComputeUptime(componentID uuid.UUID, periods []ComponentStatusHistory, createdAt, now time.Time, days int) UptimeReport {
	if days < 1 {
		days = 1
	}
	// Окно — последние `days` полных суток UTC, включая текущие.
	todayStart := now.UTC().Truncate(24 * time.Hour)
	windowStart := todayStart.AddDate(0, 0, -(days - 1))

	report := UptimeReport{ComponentID: componentID, Days: days, Daily: make([]DailyUptime, 0, days)}

	var totalEffective, totalDown time.Duration
	for d := 0; d < days; d++ {
		dayStart := windowStart.AddDate(0, 0, d)
		dayEnd := dayStart.AddDate(0, 0, 1)

		// Ограничиваем сутки моментом создания компонента и «сейчас».
		lo := maxTime(dayStart, createdAt.UTC())
		hi := minTime(dayEnd, now.UTC())
		var dayTotal, dayMaint, dayDown time.Duration
		if hi.After(lo) {
			dayTotal = hi.Sub(lo)
			for _, p := range periods {
				ps := p.StartedAt.UTC()
				pe := now.UTC()
				if p.EndedAt != nil {
					pe = p.EndedAt.UTC()
				}
				ov := overlap(lo, hi, ps, pe)
				if ov <= 0 {
					continue
				}
				switch {
				case p.Status == StatusUnderMaintenance:
					dayMaint += ov
				case statusIsDown(p.Status):
					dayDown += ov
				}
			}
		}

		effective := dayTotal - dayMaint
		dayPercent := 100.0
		if effective > 0 {
			dayPercent = float64(effective-dayDown) / float64(effective) * 100
		}
		report.Daily = append(report.Daily, DailyUptime{Date: dayStart, Percent: round2(dayPercent)})
		totalEffective += effective
		totalDown += dayDown
	}

	report.UptimePercent = 100.0
	if totalEffective > 0 {
		report.UptimePercent = round2(float64(totalEffective-totalDown) / float64(totalEffective) * 100)
	}
	return report
}

// overlap возвращает длительность пересечения интервалов [a1,a2) и [b1,b2).
func overlap(a1, a2, b1, b2 time.Time) time.Duration {
	lo := maxTime(a1, b1)
	hi := minTime(a2, b2)
	if hi.After(lo) {
		return hi.Sub(lo)
	}
	return 0
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// round2 округляет до двух знаков (проценты uptime).
func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}
