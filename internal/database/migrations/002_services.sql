CREATE TABLE services (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE service_items (
    id             TEXT PRIMARY KEY,
    service_id     TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    position       INTEGER NOT NULL,
    kind           TEXT NOT NULL,
    title          TEXT NOT NULL,
    translation_id TEXT NOT NULL,
    book_id        INTEGER NOT NULL,
    chapter        INTEGER NOT NULL,
    verse_start    INTEGER NOT NULL,
    verse_end      INTEGER NOT NULL,
    created_at     TEXT NOT NULL,
    UNIQUE (service_id, position)
);

CREATE INDEX service_items_service ON service_items (service_id, position);
