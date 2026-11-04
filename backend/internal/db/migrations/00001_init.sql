-- +goose Up

-- pgvector: тип vector и индексы для семантического поиска
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email         text NOT NULL UNIQUE,
    display_name  text NOT NULL,
    password_hash text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);

-- В базе хранится только SHA-256 от токена сессии: утечка дампа не даёт войти под пользователем
CREATE TABLE sessions (
    token_hash bytea PRIMARY KEY,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);
CREATE INDEX sessions_user_idx ON sessions (user_id);
CREATE INDEX sessions_expires_idx ON sessions (expires_at);

-- Книги, сканы, лекции: сам файл лежит на диске, здесь метаданные
CREATE TABLE documents (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id   uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title      text NOT NULL,
    kind       text NOT NULL CHECK (kind IN ('pdf', 'epub', 'markdown')),
    file_path  text NOT NULL,
    file_size  bigint NOT NULL,
    -- Хеш содержимого: один и тот же файл нельзя загрузить дважды (та самая проблема дублей)
    sha256     bytea NOT NULL,
    status     text NOT NULL DEFAULT 'uploaded'
               CHECK (status IN ('uploaded', 'processing', 'ready', 'failed')),
    error      text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (owner_id, sha256)
);
CREATE INDEX documents_owner_idx ON documents (owner_id, created_at DESC);

CREATE TABLE notes (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id       uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title          text NOT NULL DEFAULT '',
    body           text NOT NULL DEFAULT '',
    -- Если заметка — цитата, здесь источник и место в нём («стр. 42», «гл. 3»)
    document_id    uuid REFERENCES documents (id) ON DELETE SET NULL,
    quote_location text,
    -- Оптимистичная блокировка: сохранение проходит, только если версия не изменилась
    version        integer NOT NULL DEFAULT 1,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notes_owner_updated_idx ON notes (owner_id, updated_at DESC);
CREATE INDEX notes_document_idx ON notes (document_id) WHERE document_id IS NOT NULL;

CREATE TABLE tags (
    id       uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name     text NOT NULL,
    UNIQUE (owner_id, name)
);

CREATE TABLE note_tags (
    note_id uuid NOT NULL REFERENCES notes (id) ON DELETE CASCADE,
    tag_id  uuid NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
    PRIMARY KEY (note_id, tag_id)
);
CREATE INDEX note_tags_tag_idx ON note_tags (tag_id);

CREATE TABLE document_tags (
    document_id uuid NOT NULL REFERENCES documents (id) ON DELETE CASCADE,
    tag_id      uuid NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
    PRIMARY KEY (document_id, tag_id)
);
CREATE INDEX document_tags_tag_idx ON document_tags (tag_id);

-- Связи «материал ↔ материал». Полиморфные (note/document), поэтому без внешних ключей:
-- при удалении материала связи чистит триггер ниже
CREATE TABLE links (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id   uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    from_type  text NOT NULL CHECK (from_type IN ('note', 'document')),
    from_id    uuid NOT NULL,
    to_type    text NOT NULL CHECK (to_type IN ('note', 'document')),
    to_id      uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (from_type, from_id, to_type, to_id)
);
CREATE INDEX links_from_idx ON links (from_type, from_id);
CREATE INDEX links_to_idx ON links (to_type, to_id);

-- Совместный доступ: кому и с какой ролью открыт материал
CREATE TABLE shares (
    resource_type text NOT NULL CHECK (resource_type IN ('note', 'document')),
    resource_id   uuid NOT NULL,
    user_id       uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role          text NOT NULL CHECK (role IN ('viewer', 'editor')),
    created_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (resource_type, resource_id, user_id)
);
CREATE INDEX shares_user_idx ON shares (user_id);

-- Фрагменты текста для поиска: и полнотекстового (tsv), и семантического (embedding).
-- Конфигурация 'russian' стеммит русские слова, а латиницу — английским стеммером.
CREATE TABLE chunks (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    document_id uuid REFERENCES documents (id) ON DELETE CASCADE,
    note_id     uuid REFERENCES notes (id) ON DELETE CASCADE,
    position    integer NOT NULL,
    page        integer,
    content     text NOT NULL,
    tsv         tsvector GENERATED ALWAYS AS (to_tsvector('russian', content)) STORED,
    -- 1024 — размерность bge-m3; эмбеддинг заполняется позже, фоновой задачей
    embedding   vector(1024),
    CHECK ((document_id IS NULL) <> (note_id IS NULL))
);
CREATE INDEX chunks_document_idx ON chunks (document_id, position) WHERE document_id IS NOT NULL;
CREATE INDEX chunks_note_idx ON chunks (note_id, position) WHERE note_id IS NOT NULL;
CREATE INDEX chunks_tsv_idx ON chunks USING gin (tsv);
CREATE INDEX chunks_embedding_idx ON chunks USING hnsw (embedding vector_cosine_ops);

-- +goose StatementBegin
CREATE FUNCTION cleanup_polymorphic_refs() RETURNS trigger AS $$
BEGIN
    DELETE FROM links
     WHERE (from_type = TG_ARGV[0] AND from_id = OLD.id)
        OR (to_type = TG_ARGV[0] AND to_id = OLD.id);
    DELETE FROM shares
     WHERE resource_type = TG_ARGV[0] AND resource_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER notes_cleanup AFTER DELETE ON notes
    FOR EACH ROW EXECUTE FUNCTION cleanup_polymorphic_refs('note');
CREATE TRIGGER documents_cleanup AFTER DELETE ON documents
    FOR EACH ROW EXECUTE FUNCTION cleanup_polymorphic_refs('document');

-- +goose Down
DROP TABLE IF EXISTS chunks, shares, links, document_tags, note_tags, tags, notes, documents, sessions, users;
DROP FUNCTION IF EXISTS cleanup_polymorphic_refs();
