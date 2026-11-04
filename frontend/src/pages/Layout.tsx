import { useCallback, useEffect, useState } from "react";
import { Link, Outlet, useMatch, useNavigate, useOutletContext } from "react-router";
import { api, type NoteSummary, type TagCount } from "../api";
import { useAuth } from "../auth";

type LayoutContext = {
  reloadNotes: () => Promise<void>;
  createNote: () => Promise<void>;
};

export function useLayout() {
  return useOutletContext<LayoutContext>();
}

const dateFormat = new Intl.DateTimeFormat("ru-RU", { day: "numeric", month: "short" });

/** Первая строка текста без markdown-разметки — для превью в списке. */
function previewLine(snippet: string): string {
  const line = snippet.split("\n").find((l) => l.trim() !== "") ?? "";
  return line.replace(/^[#>\-*\s]+/, "").replace(/[*_`]/g, "");
}

export function Layout() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const match = useMatch("/notes/:id");
  const activeId = match?.params.id;

  const [notes, setNotes] = useState<NoteSummary[]>([]);
  const [tags, setTags] = useState<TagCount[]>([]);
  const [tag, setTag] = useState("");
  const [error, setError] = useState("");

  const reloadNotes = useCallback(async () => {
    try {
      const [n, t] = await Promise.all([api.listNotes(tag), api.listTags()]);
      setNotes(n);
      setTags(t);
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Не удалось загрузить заметки");
    }
  }, [tag]);

  useEffect(() => {
    void reloadNotes();
  }, [reloadNotes]);

  // Если выбранный тег исчез (последнюю заметку с ним удалили), сбрасываем фильтр
  useEffect(() => {
    if (tag && !tags.some((t) => t.name === tag)) setTag("");
  }, [tag, tags]);

  const createNote = useCallback(async () => {
    const note = await api.createNote({ title: "", body: "", tags: tag ? [tag] : [] });
    await reloadNotes();
    navigate(`/notes/${note.id}`);
  }, [navigate, reloadNotes, tag]);

  return (
    <div className={`shell ${activeId ? "shell--note-open" : ""}`}>
      <aside className="sidebar">
        <header className="sidebar__head">
          <Link to="/" className="brand">Wormest</Link>
          <button className="btn btn--primary" onClick={() => void createNote()}>
            Новая заметка
          </button>
        </header>

        {tags.length > 0 && (
          <nav className="tags" aria-label="Фильтр по тегам">
            <button
              className="tag"
              aria-pressed={tag === ""}
              onClick={() => setTag("")}
            >
              Все
            </button>
            {tags.map((t) => (
              <button
                key={t.name}
                className="tag"
                aria-pressed={tag === t.name}
                onClick={() => setTag(tag === t.name ? "" : t.name)}
              >
                {t.name} <span className="tag__count">{t.count}</span>
              </button>
            ))}
          </nav>
        )}

        {error && <p className="notice notice--error">{error}</p>}

        <ul className="note-list">
          {notes.map((n) => (
            <li key={n.id}>
              <Link
                to={`/notes/${n.id}`}
                className="note-item"
                aria-current={n.id === activeId ? "page" : undefined}
              >
                <span className="note-item__title">
                  <span className="note-item__mark">{n.title || "Без названия"}</span>
                </span>
                <span className="note-item__meta">
                  <time dateTime={n.updatedAt}>{dateFormat.format(new Date(n.updatedAt))}</time>
                  {previewLine(n.snippet) && <span className="note-item__snippet">{previewLine(n.snippet)}</span>}
                </span>
              </Link>
            </li>
          ))}
        </ul>

        {notes.length === 0 && !error && (
          <p className="sidebar__empty">
            {tag ? `С тегом «${tag}» заметок нет.` : "Заметок пока нет. Создайте первую."}
          </p>
        )}

        <footer className="sidebar__foot">
          <span className="sidebar__user">{user?.displayName}</span>
          <button className="btn btn--quiet" onClick={() => void logout()}>Выйти</button>
        </footer>
      </aside>

      <main className="main">
        <Outlet context={{ reloadNotes, createNote } satisfies LayoutContext} />
      </main>
    </div>
  );
}
