package domain

import (
	"sort"

	"github.com/google/uuid"
)

// ComponentNode — узел дерева компонентов (DESIGN §3.2: подкомпоненты через parent_id).
type ComponentNode struct {
	Component
	Children []*ComponentNode
}

// BuildComponentTree строит лес из плоского списка компонентов по ParentID.
// Корни — компоненты без родителя (или чей родитель отсутствует в списке). Дети и корни
// сортируются по Position, затем по Name — стабильный порядок отображения.
// Возможные циклы parent_id в результат не попадают (узел добавляется ребёнком не более раза).
func BuildComponentTree(components []Component) []*ComponentNode {
	nodes := make(map[uuid.UUID]*ComponentNode, len(components))
	for _, c := range components {
		nodes[c.ID] = &ComponentNode{Component: c}
	}

	var roots []*ComponentNode
	for _, c := range components {
		node := nodes[c.ID]
		if c.ParentID != nil {
			if parent, ok := nodes[*c.ParentID]; ok && parent != node {
				parent.Children = append(parent.Children, node)
				continue
			}
		}
		roots = append(roots, node)
	}

	sortNodes(roots)
	for _, n := range nodes {
		sortNodes(n.Children)
	}
	return roots
}

func sortNodes(ns []*ComponentNode) {
	sort.SliceStable(ns, func(i, j int) bool {
		if ns[i].Position != ns[j].Position {
			return ns[i].Position < ns[j].Position
		}
		return ns[i].Name < ns[j].Name
	})
}

// EffectiveStatus — статус узла с учётом всех потомков: худший (по приоритету показа)
// из собственного статуса и эффективных статусов дочерних узлов (DESIGN §6).
func (n *ComponentNode) EffectiveStatus() ComponentStatus {
	worst := n.CurrentStatus
	for _, child := range n.Children {
		worst = WorstStatus(worst, child.EffectiveStatus())
	}
	return worst
}
