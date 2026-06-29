import { useCallback, useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";

import { buildComponentTree, listComponents } from "@/entities/component";
import type { Component } from "@/entities/component";
import { deleteGroup, listGroups } from "@/entities/componentGroup";
import type { ComponentGroup } from "@/entities/componentGroup";
import { getPage } from "@/entities/page";
import type { StatusPage } from "@/entities/page";
import { CreateComponentForm } from "@/features/component-create";
import { CreateGroupForm } from "@/features/group-create";
import { HttpError } from "@/shared/api";
import { Button, Card } from "@/shared/ui";
import { ComponentTree } from "@/widgets/component-tree";
import { PageNav } from "@/widgets/page-nav";

export function PageDetailPage() {
  const { id = "" } = useParams();
  const [page, setPage] = useState<StatusPage | null>(null);
  const [groups, setGroups] = useState<ComponentGroup[]>([]);
  const [components, setComponents] = useState<Component[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const reloadComponents = useCallback(() => {
    return listComponents(id).then(setComponents);
  }, [id]);

  const reloadGroups = useCallback(() => {
    return listGroups(id).then(setGroups);
  }, [id]);

  useEffect(() => {
    setLoading(true);
    Promise.all([getPage(id), listGroups(id), listComponents(id)])
      .then(([p, g, c]) => {
        setPage(p);
        setGroups(g);
        setComponents(c);
      })
      .catch((err) =>
        setError(err instanceof HttpError ? err.message : "Не удалось загрузить страницу"),
      )
      .finally(() => setLoading(false));
  }, [id]);

  function patchComponent(updated: Component) {
    setComponents((prev) => prev.map((c) => (c.id === updated.id ? updated : c)));
  }

  async function handleDeleteGroup(group: ComponentGroup) {
    if (!window.confirm(`Удалить группу «${group.name}»? Её компоненты станут без группы.`))
      return;
    try {
      await deleteGroup(group.id);
      await Promise.all([reloadGroups(), reloadComponents()]);
    } catch {
      window.alert("Не удалось удалить группу");
    }
  }

  if (loading) return <div className="hp-container hp-muted">Загрузка…</div>;
  if (error) return <div className="hp-container hp-error">{error}</div>;
  if (!page) return null;

  const ungrouped = components.filter((c) => !c.group_id);

  return (
    <div className="hp-container">
      <div style={{ marginBottom: 16 }}>
        <Link to="/" className="hp-muted">
          ← Все страницы
        </Link>
        <h1 style={{ marginTop: 8 }}>{page.name}</h1>
        <div className="hp-muted" style={{ fontSize: 13 }}>
          /{page.slug} · {page.visibility === "private" ? "приватная" : "публичная"}
        </div>
        <PageNav pageId={id} />
      </div>

      <Card>
        <h2>Группы и компоненты</h2>
        <CreateGroupForm pageId={id} onCreated={(g) => setGroups((prev) => [...prev, g])} />
      </Card>

      {groups.map((group) => {
        const groupNodes = buildComponentTree(
          components.filter((c) => c.group_id === group.id),
        );
        return (
          <Card key={group.id}>
            <div className="hp-card__header" style={{ marginBottom: 8 }}>
              <h3>{group.name}</h3>
              <Button
                variant="danger"
                size="sm"
                onClick={() => handleDeleteGroup(group)}
              >
                Удалить группу
              </Button>
            </div>
            <ComponentTree
              nodes={groupNodes}
              onChanged={patchComponent}
              onDeleted={() => reloadComponents()}
            />
          </Card>
        );
      })}

      <Card>
        <div className="hp-card__header" style={{ marginBottom: 8 }}>
          <h3>Без группы</h3>
        </div>
        <ComponentTree
          nodes={buildComponentTree(ungrouped)}
          onChanged={patchComponent}
          onDeleted={() => reloadComponents()}
        />
      </Card>

      <Card>
        <CreateComponentForm
          pageId={id}
          groups={groups}
          components={components}
          onCreated={() => reloadComponents()}
        />
      </Card>
    </div>
  );
}
