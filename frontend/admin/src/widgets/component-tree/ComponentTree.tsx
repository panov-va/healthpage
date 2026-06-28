import { deleteComponent } from "@/entities/component";
import type { Component, ComponentNode } from "@/entities/component";
import { StatusSelect } from "@/features/component-status";
import { Button } from "@/shared/ui";

// Рекурсивный рендер дерева компонентов: статус-селект + удаление + вложенность
// по parent_id. Приватные компоненты помечаются меткой (на публичной странице скрыты).
export function ComponentTree({
  nodes,
  onChanged,
  onDeleted,
}: {
  nodes: ComponentNode[];
  onChanged: (component: Component) => void;
  onDeleted: (id: string) => void;
}) {
  if (nodes.length === 0) {
    return <p className="hp-muted">Пока нет компонентов.</p>;
  }
  return (
    <div>
      {nodes.map((node) => (
        <ComponentRow
          key={node.component.id}
          node={node}
          onChanged={onChanged}
          onDeleted={onDeleted}
        />
      ))}
    </div>
  );
}

function ComponentRow({
  node,
  onChanged,
  onDeleted,
}: {
  node: ComponentNode;
  onChanged: (component: Component) => void;
  onDeleted: (id: string) => void;
}) {
  const { component, children } = node;

  async function handleDelete() {
    if (!window.confirm(`Удалить компонент «${component.name}»?`)) return;
    try {
      await deleteComponent(component.id);
      onDeleted(component.id);
    } catch {
      window.alert("Не удалось удалить компонент");
    }
  }

  return (
    <div>
      <div className="hp-list-item">
        <div>
          <span style={{ fontWeight: 500 }}>{component.name}</span>
          {component.is_private && (
            <span className="hp-muted" style={{ fontSize: 12, marginLeft: 8 }}>
              приватный
            </span>
          )}
        </div>
        <div className="hp-row">
          <StatusSelect component={component} onChanged={onChanged} />
          <Button variant="danger" size="sm" onClick={handleDelete}>
            Удалить
          </Button>
        </div>
      </div>
      {children.length > 0 && (
        <div className="hp-subtree">
          {children.map((child) => (
            <ComponentRow
              key={child.component.id}
              node={child}
              onChanged={onChanged}
              onDeleted={onDeleted}
            />
          ))}
        </div>
      )}
    </div>
  );
}
