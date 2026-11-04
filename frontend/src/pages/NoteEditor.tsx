import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router";
import CodeMirror from "@uiw/react-codemirror";
import { markdown } from "@codemirror/lang-markdown";
import { EditorView } from "@codemirror/view";
import { api, conflictNote, ApiError, type Note } from "../api";
import { markdownHighlighting } from "../editorTheme";
import { useLayout } from "./Layout";

// Расширения создаются один раз: иначе CodeMirror пересоздавал бы их на каждый рендер
const extensions = [markdown(), markdownHighlighting, EditorView.lineWrapping];

const AUTOSAVE_MS = 800;

type Status = "loading" | "saved" | "dirty" | "saving" | "conflict" | "error" | "missing";

type Draft = { title: string; body: string; tagsText: string };

const parseTags = (text: string) =>
  text.split(",").map((t) => t.trim()).filter(Boolean);

const toDraft = (n: Note): Draft => ({ title: n.title, body: n.body, tagsText: n.tags.join(", ") });

export function NoteEditor() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const { reloadNotes } = useLayout();

  const [draft, setDraft] = useState<Draft>({ title: "", body: "", tagsText: "" });
  const [status, setStatus] = useState<Status>("loading");
  const [serverNote, setServerNote] = useState<Note | null>(null);
  const [errorText, setErrorText] = useState("");

  // Refs — чтобы таймер автосохранения и запросы видели свежие значения, а не замыкание
  const draftRef = useRef(draft);
  const versionRef = useRef(0);
  const timerRef = useRef<number | undefined>(undefined);
  const savingRef = useRef(false);
  const queuedRef = useRef(false);
  const blockedRef = useRef(false); // при конфликте автосохранение останавливается

  // Сохранения идут строго по очереди: два параллельных PUT с одной версией
  // дали бы конфликт с самим собой
  const save = useCallback(async (noteId: string, forceVersion?: number) => {
    if (savingRef.current) {
      queuedRef.current = true;
      return;
    }
    savingRef.current = true;
    setStatus("saving");
    const d = draftRef.current;
    try {
      const saved = await api.updateNote(
        noteId,
        { title: d.title, body: d.body, tags: parseTags(d.tagsText) },
        forceVersion ?? versionRef.current,
      );
      versionRef.current = saved.version;
      setStatus(queuedRef.current ? "dirty" : "saved");
      void reloadNotes();
    } catch (err) {
      const current = conflictNote(err);
      if (current) {
        blockedRef.current = true;
        queuedRef.current = false;
        setServerNote(current);
        setStatus("conflict");
      } else {
        setErrorText(err instanceof Error ? err.message : "Не удалось сохранить");
        setStatus("error");
      }
    } finally {
      savingRef.current = false;
      if (queuedRef.current && !blockedRef.current) {
        queuedRef.current = false;
        void save(noteId);
      }
    }
  }, [reloadNotes]);

  // Загрузка заметки при смене id
  useEffect(() => {
    let cancelled = false;
    setStatus("loading");
    setServerNote(null);
    blockedRef.current = false;

    api.getNote(id)
      .then((note) => {
        if (cancelled) return;
        const d = toDraft(note);
        draftRef.current = d;
        versionRef.current = note.version;
        setDraft(d);
        setStatus("saved");
      })
      .catch((err) => {
        if (cancelled) return;
        if (err instanceof ApiError && err.status === 404) setStatus("missing");
        else {
          setErrorText(err instanceof Error ? err.message : "Не удалось открыть заметку");
          setStatus("error");
        }
      });

    return () => {
      cancelled = true;
      // Уходим с заметки, а таймер ещё тикает — сохраняем сразу, чтобы не потерять правку
      if (timerRef.current !== undefined) {
        window.clearTimeout(timerRef.current);
        timerRef.current = undefined;
        if (!blockedRef.current) void save(id);
      }
    };
  }, [id, save]);

  // Предупреждение при закрытии вкладки с несохранёнными правками
  useEffect(() => {
    const onBeforeUnload = (e: BeforeUnloadEvent) => {
      if (timerRef.current !== undefined || savingRef.current) e.preventDefault();
    };
    window.addEventListener("beforeunload", onBeforeUnload);
    return () => window.removeEventListener("beforeunload", onBeforeUnload);
  }, []);

  function update(patch: Partial<Draft>) {
    const next = { ...draftRef.current, ...patch };
    draftRef.current = next;
    setDraft(next);
    if (blockedRef.current) return;
    setStatus("dirty");
    window.clearTimeout(timerRef.current);
    timerRef.current = window.setTimeout(() => {
      timerRef.current = undefined;
      void save(id);
    }, AUTOSAVE_MS);
  }

  function takeServerVersion() {
    if (!serverNote) return;
    const d = toDraft(serverNote);
    draftRef.current = d;
    versionRef.current = serverNote.version;
    blockedRef.current = false;
    setDraft(d);
    setServerNote(null);
    setStatus("saved");
  }

  function keepMine() {
    if (!serverNote) return;
    blockedRef.current = false;
    const v = serverNote.version;
    setServerNote(null);
    void save(id, v);
  }

  async function remove() {
    if (!window.confirm("Удалить заметку? Отменить это нельзя.")) return;
    window.clearTimeout(timerRef.current);
    timerRef.current = undefined;
    blockedRef.current = true;
    try {
      await api.deleteNote(id);
      await reloadNotes();
      navigate("/");
    } catch (err) {
      blockedRef.current = false;
      setErrorText(err instanceof Error ? err.message : "Не удалось удалить");
      setStatus("error");
    }
  }

  if (status === "missing") {
    return (
      <div className="empty">
        <p>Такой заметки нет — возможно, её удалили на другом устройстве.</p>
        <Link className="btn" to="/">К списку</Link>
      </div>
    );
  }

  return (
    <article className="editor" aria-busy={status === "loading"}>
      <div className="editor__bar">
        <Link to="/" className="editor__back">Все заметки</Link>
        <span className="editor__status" role="status">{statusText(status)}</span>
        <button className="btn btn--quiet" onClick={() => void remove()}>Удалить</button>
      </div>

      {status === "conflict" && serverNote && (
        <div className="notice notice--conflict" role="alert">
          <p>Эту заметку изменили на другом устройстве, пока вы её правили.</p>
          <div className="notice__actions">
            <button className="btn" onClick={takeServerVersion}>Открыть сохранённую версию</button>
            <button className="btn btn--primary" onClick={keepMine}>Сохранить мою поверх</button>
          </div>
        </div>
      )}
      {status === "error" && errorText && (
        <p className="notice notice--error" role="alert">{errorText}</p>
      )}

      <textarea
        className="editor__title"
        rows={1}
        placeholder="Без названия"
        value={draft.title}
        // Заголовок — одна строка: Enter не вставляет перевод строки
        onKeyDown={(e) => e.key === "Enter" && e.preventDefault()}
        onChange={(e) => update({ title: e.target.value.replace(/\n/g, " ") })}
        aria-label="Заголовок"
      />
      <input
        className="editor__tags"
        placeholder="Теги через запятую"
        value={draft.tagsText}
        onChange={(e) => update({ tagsText: e.target.value })}
        aria-label="Теги"
      />
      <CodeMirror
        className="editor__body"
        value={draft.body}
        onChange={(value) => update({ body: value })}
        extensions={extensions}
        basicSetup={{
          lineNumbers: false,
          foldGutter: false,
          highlightActiveLine: false,
          highlightActiveLineGutter: false,
          syntaxHighlighting: false, // свой стиль из editorTheme.ts
        }}
        placeholder="Пишите в markdown…"
        theme="none"
      />
    </article>
  );
}

function statusText(s: Status): string {
  switch (s) {
    case "loading": return "Открываю…";
    case "dirty": return "Есть правки";
    case "saving": return "Сохраняю…";
    case "saved": return "Сохранено";
    case "conflict": return "Конфликт версий";
    case "error": return "Ошибка";
    default: return "";
  }
}
