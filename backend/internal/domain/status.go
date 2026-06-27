package domain

// ComponentStatus — состояние компонента (нормативный enum, DESIGN §3.2 / openapi ComponentStatus).
type ComponentStatus string

const (
	StatusOperational         ComponentStatus = "operational"
	StatusDegradedPerformance ComponentStatus = "degraded_performance"
	StatusPartialOutage       ComponentStatus = "partial_outage"
	StatusMajorOutage         ComponentStatus = "major_outage"
	StatusUnderMaintenance    ComponentStatus = "under_maintenance"
)

// AllComponentStatuses — все допустимые значения (для валидации/перебора).
var AllComponentStatuses = []ComponentStatus{
	StatusOperational,
	StatusDegradedPerformance,
	StatusPartialOutage,
	StatusMajorOutage,
	StatusUnderMaintenance,
}

// IsValid сообщает, входит ли значение в нормативный enum.
func (s ComponentStatus) IsValid() bool {
	switch s {
	case StatusOperational, StatusDegradedPerformance, StatusPartialOutage,
		StatusMajorOutage, StatusUnderMaintenance:
		return true
	default:
		return false
	}
}

// displaySeverity — приоритет ПОКАЗА статуса при агрегации (DESIGN §6, [РЕШЕНО]).
//
// Базовая лесенка сбоев: operational < degraded < partial < major. Особое правило:
// under_maintenance показывается ВЫШЕ деградации (перекрывает её, т.к. деградация может быть
// следствием самих работ), но НИЖЕ реальных сбоев (плановые работы показываются «без сбоев»,
// поэтому partial/major их перекрывают). Отсюда порядок:
//
//	operational(0) < degraded_performance(1) < under_maintenance(2) < partial_outage(3) < major_outage(4)
//
// Неизвестное значение получает -1 и при агрегации игнорируется в пользу валидных.
func displaySeverity(s ComponentStatus) int {
	switch s {
	case StatusOperational:
		return 0
	case StatusDegradedPerformance:
		return 1
	case StatusUnderMaintenance:
		return 2
	case StatusPartialOutage:
		return 3
	case StatusMajorOutage:
		return 4
	default:
		return -1
	}
}

// WorstStatus возвращает статус с наивысшим приоритетом показа (DESIGN §6).
// Для пустого набора — operational («все системы работают» вакуумно).
func WorstStatus(statuses ...ComponentStatus) ComponentStatus {
	worst := StatusOperational
	worstSev := 0
	for _, s := range statuses {
		if sev := displaySeverity(s); sev > worstSev {
			worst, worstSev = s, sev
		}
	}
	return worst
}
