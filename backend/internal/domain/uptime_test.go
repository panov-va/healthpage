package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func hist(status ComponentStatus, start time.Time, end *time.Time) ComponentStatusHistory {
	return ComponentStatusHistory{Status: status, StartedAt: start, EndedAt: end}
}

func TestComputeUptimeNoHistory(t *testing.T) {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	created := now.AddDate(0, 0, -100)
	r := ComputeUptime(uuid.New(), nil, created, now, 90)
	if r.UptimePercent != 100 {
		t.Fatalf("no history → 100%%, got %v", r.UptimePercent)
	}
	if len(r.Daily) != 90 {
		t.Fatalf("daily len=%d want 90", len(r.Daily))
	}
}

func TestComputeUptimeFullDayOutage(t *testing.T) {
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	created := now.AddDate(0, 0, -30)
	// Один полный день (2026-07-05) в major_outage.
	dayStart := time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.AddDate(0, 0, 1)
	periods := []ComponentStatusHistory{hist(StatusMajorOutage, dayStart, &dayEnd)}
	r := ComputeUptime(uuid.New(), periods, created, now, 10)
	// now=07-10 00:00 → сегодняшние сутки (07-10) нулевой длины; эффективны 9 суток (07-01..07-09),
	// из них 1 полный день простоя → 8/9 = 88.89%.
	if r.UptimePercent != 88.89 {
		t.Fatalf("uptime=%v want 88.89", r.UptimePercent)
	}
	// В сам день простоя — 0%.
	for _, d := range r.Daily {
		if d.Date.Equal(dayStart) && d.Percent != 0 {
			t.Fatalf("outage day percent=%v want 0", d.Percent)
		}
	}
}

func TestComputeUptimeMaintenanceExcluded(t *testing.T) {
	now := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	created := now.AddDate(0, 0, -10)
	// Весь день 2026-07-01 под обслуживанием → исключается (не влияет на uptime).
	dayStart := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.AddDate(0, 0, 1)
	periods := []ComponentStatusHistory{hist(StatusUnderMaintenance, dayStart, &dayEnd)}
	// days=2: сутки 07-01 (полностью обслуживание) + 07-02 (нулевые). Обслуживание исключено.
	r := ComputeUptime(uuid.New(), periods, created, now, 2)
	if r.UptimePercent != 100 {
		t.Fatalf("maintenance-only window → 100%% (excluded), got %v", r.UptimePercent)
	}
}

func TestComputeUptimeMaintenancePlusOutageSameDay(t *testing.T) {
	now := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	created := now.AddDate(0, 0, -10)
	day := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	// 12ч обслуживания (исключается) + 6ч простоя из оставшихся 12ч → 6/12 = 50%.
	maintEnd := day.Add(12 * time.Hour)
	downStart := day.Add(12 * time.Hour)
	downEnd := day.Add(18 * time.Hour)
	periods := []ComponentStatusHistory{
		hist(StatusUnderMaintenance, day, &maintEnd),
		hist(StatusMajorOutage, downStart, &downEnd),
	}
	r := ComputeUptime(uuid.New(), periods, created, now, 2)
	if r.UptimePercent != 50 {
		t.Fatalf("uptime=%v want 50 (12h maint excluded, 6h down of 12h effective)", r.UptimePercent)
	}
}

func TestComputeUptimeDegradedCountsAsUp(t *testing.T) {
	now := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	created := now.AddDate(0, 0, -10)
	day := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	dayEnd := day.AddDate(0, 0, 1)
	periods := []ComponentStatusHistory{hist(StatusDegradedPerformance, day, &dayEnd)}
	r := ComputeUptime(uuid.New(), periods, created, now, 2)
	if r.UptimePercent != 100 {
		t.Fatalf("degraded → available (100%%), got %v", r.UptimePercent)
	}
}

func TestComputeUptimeOpenPeriodToNow(t *testing.T) {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	created := now.AddDate(0, 0, -10)
	// Открытый период простоя, начавшийся 6ч назад, длится до now.
	start := now.Add(-6 * time.Hour)
	periods := []ComponentStatusHistory{hist(StatusMajorOutage, start, nil)}
	r := ComputeUptime(uuid.New(), periods, created, now, 1)
	// Текущие сутки: с 00:00 до 12:00 = 12ч, из них 6ч простоя → 50%.
	if r.UptimePercent != 50 {
		t.Fatalf("open outage period uptime=%v want 50", r.UptimePercent)
	}
}
