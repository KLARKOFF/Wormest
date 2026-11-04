import { useLayout } from "./Layout";

export function EmptyState() {
  const { createNote } = useLayout();
  return (
    <div className="empty">
      <p>Откройте заметку из списка или начните новую.</p>
      <button className="btn" onClick={() => void createNote()}>Новая заметка</button>
    </div>
  );
}
