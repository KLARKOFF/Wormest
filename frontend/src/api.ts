// Тонкая обёртка над fetch и типы ответов бэкенда.

export type User = {
  id: string;
  email: string;
  displayName: string;
  createdAt: string;
};

export type Note = {
  id: string;
  ownerId: string;
  title: string;
  body: string;
  documentId: string | null;
  quoteLocation: string | null;
  tags: string[];
  version: number;
  createdAt: string;
  updatedAt: string;
};

export type NoteSummary = Pick<Note, "id" | "title" | "tags" | "updatedAt"> & {
  snippet: string;
};

export type NoteInput = {
  title: string;
  body: string;
  tags: string[];
  documentId?: string | null;
  quoteLocation?: string | null;
};

export type TagCount = { name: string; count: number };

export class ApiError extends Error {
  status: number;
  data: unknown;

  constructor(status: number, message: string, data: unknown) {
    super(message);
    this.status = status;
    this.data = data;
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`/api${path}`, {
    method,
    credentials: "same-origin",
    headers: body !== undefined ? { "Content-Type": "application/json" } : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });

  if (res.status === 204) return undefined as T;

  const data = await res.json().catch(() => null);
  if (!res.ok) {
    const message =
      data && typeof data === "object" && "error" in data
        ? String((data as { error: unknown }).error)
        : `Сервер ответил ${res.status}`;
    throw new ApiError(res.status, message, data);
  }
  return data as T;
}

export const api = {
  me: () => request<User>("GET", "/auth/me"),
  login: (email: string, password: string) =>
    request<User>("POST", "/auth/login", { email, password }),
  register: (email: string, password: string, displayName: string) =>
    request<User>("POST", "/auth/register", { email, password, displayName }),
  logout: () => request<void>("POST", "/auth/logout"),

  listNotes: (tag?: string) =>
    request<NoteSummary[]>("GET", tag ? `/notes?tag=${encodeURIComponent(tag)}` : "/notes"),
  getNote: (id: string) => request<Note>("GET", `/notes/${id}`),
  createNote: (input: NoteInput) => request<Note>("POST", "/notes", input),
  updateNote: (id: string, input: NoteInput, version: number) =>
    request<Note>("PUT", `/notes/${id}`, { ...input, version }),
  deleteNote: (id: string) => request<void>("DELETE", `/notes/${id}`),

  listTags: () => request<TagCount[]>("GET", "/tags"),
};

/** При 409 бэкенд кладёт в ответ актуальную версию заметки. */
export function conflictNote(err: unknown): Note | null {
  if (err instanceof ApiError && err.status === 409) {
    const data = err.data as { note?: Note } | null;
    return data?.note ?? null;
  }
  return null;
}
