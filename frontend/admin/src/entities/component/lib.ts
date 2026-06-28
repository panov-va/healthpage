import type { Component } from "@/shared/api";

export interface ComponentNode {
  component: Component;
  children: ComponentNode[];
}

// Строит лес компонентов по parent_id. Сортировка: position → name.
// Безопасно к отсутствующим родителям (такой компонент становится корнем).
export function buildComponentTree(components: Component[]): ComponentNode[] {
  const byId = new Map<string, ComponentNode>();
  for (const c of components) byId.set(c.id, { component: c, children: [] });

  const roots: ComponentNode[] = [];
  for (const node of byId.values()) {
    const parentId = node.component.parent_id;
    const parent = parentId ? byId.get(parentId) : undefined;
    if (parent) parent.children.push(node);
    else roots.push(node);
  }

  const sortNodes = (nodes: ComponentNode[]) => {
    nodes.sort((a, b) => {
      const pa = a.component.position ?? 0;
      const pb = b.component.position ?? 0;
      if (pa !== pb) return pa - pb;
      return a.component.name.localeCompare(b.component.name);
    });
    for (const n of nodes) sortNodes(n.children);
  };
  sortNodes(roots);
  return roots;
}
