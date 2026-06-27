package domain

import "github.com/google/uuid"

// ComputeOverallStatus — общий статус страницы: худший (по приоритету показа, DESIGN §6)
// среди компонентов, влияющих на статус (публичные и с display_state=true). Приватные и
// информационные метки игнорируются. Пустой набор → operational.
func ComputeOverallStatus(components []Component) ComponentStatus {
	statuses := make([]ComponentStatus, 0, len(components))
	for _, c := range components {
		if c.CountsTowardStatus() {
			statuses = append(statuses, c.CurrentStatus)
		}
	}
	return WorstStatus(statuses...)
}

// ComputeGroupStatus — агрегированный статус группы: худший среди компонентов группы,
// влияющих на статус (DESIGN §6). Учитываются компоненты с GroupID == groupID.
func ComputeGroupStatus(groupID uuid.UUID, components []Component) ComponentStatus {
	statuses := make([]ComponentStatus, 0, len(components))
	for _, c := range components {
		if c.GroupID != nil && *c.GroupID == groupID && c.CountsTowardStatus() {
			statuses = append(statuses, c.CurrentStatus)
		}
	}
	return WorstStatus(statuses...)
}
