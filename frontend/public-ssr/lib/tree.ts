// Сборка дерева компонентов из плоского списка (подкомпоненты через parent_id — DESIGN §3.2).
// Backend отдаёт компоненты плоским списком (по position, name); вложенность строим на клиенте.

import type { ApiComponent } from "./api";

export interface ComponentNode {
  component: ApiComponent;
  children: ComponentNode[];
}

// buildTree собирает лес узлов из набора компонентов (например, одной группы или ungrouped).
// Родитель за пределами набора (или отсутствующий) трактуется как корень — узел не теряется.
export function buildTree(components: ApiComponent[]): ComponentNode[] {
  const byID = new Map<string, ComponentNode>();
  for (const c of components) {
    byID.set(c.id, { component: c, children: [] });
  }

  const roots: ComponentNode[] = [];
  for (const node of byID.values()) {
    const parentID = node.component.parent_id;
    const parent = parentID ? byID.get(parentID) : undefined;
    if (parent) {
      parent.children.push(node);
    } else {
      roots.push(node);
    }
  }
  return roots;
}
