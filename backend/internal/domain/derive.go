package domain

import "github.com/google/uuid"

// DerivedComponentStatus вычисляет статус, который активные инциденты и плановые работы
// навязывают компоненту (DESIGN §3.3, §3.4, §6), и сообщает, навязан ли он вообще.
//
// Берётся худший по приоритету показа (§6, WorstStatus) среди:
//   - статусов, которые активные инциденты (Incident.IsActive) назначили этому компоненту в
//     своих отчётах (IncidentComponent.ComponentStatusInIncident);
//   - under_maintenance, если компонент затронут активной (in_progress) работой
//     (Maintenance.ImposedComponentStatus).
//
// Завершённые (resolved) инциденты и не идущие работы вклад не вносят; soft-deleted записи
// игнорируются. Если ни один активный инцидент/работа не затрагивает компонент — возвращается
// (operational, false): по §3.3 компонент возвращается в operational. Флаг driven=false
// сигнализирует store/service, что авто-статус снят и компонент снова управляется вручную, —
// это позволяет сохранить ручной статус оператора («если оператор не указал иное», §3.3).
func DerivedComponentStatus(
	componentID uuid.UUID,
	incidents []Incident,
	maintenances []Maintenance,
) (status ComponentStatus, driven bool) {
	imposed := make([]ComponentStatus, 0)

	for _, inc := range incidents {
		if inc.DeletedAt != nil || !inc.IsActive() {
			continue
		}
		for _, ic := range inc.Components {
			if ic.ComponentID == componentID {
				imposed = append(imposed, ic.ComponentStatusInIncident)
			}
		}
	}

	for _, m := range maintenances {
		if m.DeletedAt != nil {
			continue
		}
		ms, ok := m.ImposedComponentStatus()
		if !ok {
			continue
		}
		for _, cid := range m.ComponentIDs {
			if cid == componentID {
				imposed = append(imposed, ms)
				break
			}
		}
	}

	if len(imposed) == 0 {
		return StatusOperational, false
	}
	return WorstStatus(imposed...), true
}
