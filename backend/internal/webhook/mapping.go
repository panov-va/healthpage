package webhook

import (
	"encoding/json"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
)

// Mapping — настройка маппинга алерта на компоненты (component_mapping интеграции).
//   - Map: ключ → component_id (uuid строкой). Ключ берётся из метки MatchLabel алерта.
//   - DefaultComponentIDs: компоненты, если по метке совпадения нет.
//   - MatchLabel: имя метки алерта, чьё значение используется как ключ Map.
//   - DefaultImpact: impact создаваемого инцидента (по умолчанию major).
type Mapping struct {
	Map                 map[string]string `json:"map"`
	DefaultComponentIDs []string          `json:"default_component_ids"`
	MatchLabel          string            `json:"match_label"`
	DefaultImpact       string            `json:"default_impact"`
}

// ParseMapping разбирает component_mapping (jsonb). Пустой/нулевой вход → пустой маппинг.
func ParseMapping(raw []byte) (Mapping, error) {
	var m Mapping
	if len(raw) == 0 || string(raw) == "null" {
		return m, nil
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return Mapping{}, err
	}
	return m, nil
}

// Impact возвращает impact для инцидента: DefaultImpact, если валиден, иначе major.
func (m Mapping) Impact() domain.IncidentImpact {
	imp := domain.IncidentImpact(m.DefaultImpact)
	if imp.IsValid() {
		return imp
	}
	return domain.ImpactMajor
}

// Resolve определяет компоненты, затронутые алертом: по метке MatchLabel ищет ключ в Map,
// иначе берёт DefaultComponentIDs. Невалидные uuid пропускаются; порядок сохраняется, дубли убираются.
func (m Mapping) Resolve(a Alert) []uuid.UUID {
	var raw []string
	if m.MatchLabel != "" {
		if key := a.Labels[m.MatchLabel]; key != "" {
			if id, ok := m.Map[key]; ok {
				raw = []string{id}
			}
		}
	}
	if raw == nil {
		raw = m.DefaultComponentIDs
	}
	seen := make(map[uuid.UUID]bool, len(raw))
	out := make([]uuid.UUID, 0, len(raw))
	for _, s := range raw {
		id, err := uuid.Parse(s)
		if err != nil || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
