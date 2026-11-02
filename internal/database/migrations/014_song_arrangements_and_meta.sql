ALTER TABLE songs ADD COLUMN ccli_number TEXT NOT NULL DEFAULT '';
ALTER TABLE songs ADD COLUMN source TEXT NOT NULL DEFAULT '';

CREATE TABLE song_arrangements (
    id         TEXT PRIMARY KEY,
    song_id    TEXT NOT NULL REFERENCES songs(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    is_default INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE song_arrangement_items (
    id             TEXT PRIMARY KEY,
    arrangement_id TEXT NOT NULL REFERENCES song_arrangements(id) ON DELETE CASCADE,
    section_id     TEXT NOT NULL REFERENCES song_sections(id) ON DELETE CASCADE,
    position       INTEGER NOT NULL,
    UNIQUE (arrangement_id, position)
);
