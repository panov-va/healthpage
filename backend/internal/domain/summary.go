package domain

// GroupSummary — группа в публичной сводке: агрегированный статус и её публичные компоненты.
type GroupSummary struct {
	Group            ComponentGroup
	AggregatedStatus ComponentStatus
	Components       []Component
}

// PublicSummary — данные публичной сводки страницы (без инцидентов/работ — они на этапе 2).
type PublicSummary struct {
	OverallStatus ComponentStatus
	Groups        []GroupSummary
	Ungrouped     []Component
}

// BuildPublicSummary собирает публичную сводку (DESIGN §6):
//   - приватные компоненты не показываются и не влияют на статус;
//   - общий статус — худший по приоритету показа среди влияющих компонентов;
//   - статус группы — худший среди её компонентов.
//
// Порядок групп и компонентов сохраняется как во входных срезах (store отдаёт по position, name).
func BuildPublicSummary(groups []ComponentGroup, components []Component) PublicSummary {
	// Видимые публично компоненты (приватные скрыты целиком; display_state=false показываются
	// как информационные метки — на статус не влияют, это учитывает Compute*Status).
	visible := make([]Component, 0, len(components))
	for _, c := range components {
		if !c.IsPrivate {
			visible = append(visible, c)
		}
	}

	summary := PublicSummary{
		OverallStatus: ComputeOverallStatus(components),
		Groups:        make([]GroupSummary, 0, len(groups)),
	}

	for _, g := range groups {
		gc := make([]Component, 0)
		for _, c := range visible {
			if c.GroupID != nil && *c.GroupID == g.ID {
				gc = append(gc, c)
			}
		}
		summary.Groups = append(summary.Groups, GroupSummary{
			Group:            g,
			AggregatedStatus: ComputeGroupStatus(g.ID, components),
			Components:       gc,
		})
	}

	for _, c := range visible {
		if c.GroupID == nil {
			summary.Ungrouped = append(summary.Ungrouped, c)
		}
	}
	return summary
}
