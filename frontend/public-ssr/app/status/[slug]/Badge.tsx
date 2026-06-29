// Бейдж-«пилюля» с цветной точкой — для статусов/влияния инцидентов и работ (этап 2.10).

export function Badge({ label, color }: { label: string; color: string }) {
  return (
    <span className="badge">
      <span className="badge-dot" style={{ background: color }} aria-hidden="true" />
      {label}
    </span>
  );
}
