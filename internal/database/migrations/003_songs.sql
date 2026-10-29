ALTER TABLE service_items ADD COLUMN song_id TEXT NOT NULL DEFAULT '';
ALTER TABLE service_items ADD COLUMN section_id TEXT NOT NULL DEFAULT '';

CREATE TABLE songs (
    id         TEXT PRIMARY KEY,
    title      TEXT NOT NULL,
    author     TEXT NOT NULL DEFAULT '',
    copyright  TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE song_sections (
    id       TEXT PRIMARY KEY,
    song_id  TEXT NOT NULL REFERENCES songs(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    label    TEXT NOT NULL,
    text     TEXT NOT NULL,
    UNIQUE (song_id, position)
);

CREATE VIRTUAL TABLE song_sections_fts USING fts5(
    text,
    label,
    content='song_sections',
    content_rowid='rowid',
    tokenize="unicode61 remove_diacritics 2"
);

CREATE TRIGGER song_sections_after_insert AFTER INSERT ON song_sections BEGIN
    INSERT INTO song_sections_fts (rowid, text, label) VALUES (new.rowid, new.text, new.label);
END;

CREATE TRIGGER song_sections_after_delete AFTER DELETE ON song_sections BEGIN
    INSERT INTO song_sections_fts (song_sections_fts, rowid, text, label) VALUES ('delete', old.rowid, old.text, old.label);
END;

CREATE TRIGGER song_sections_after_update AFTER UPDATE ON song_sections BEGIN
    INSERT INTO song_sections_fts (song_sections_fts, rowid, text, label) VALUES ('delete', old.rowid, old.text, old.label);
    INSERT INTO song_sections_fts (rowid, text, label) VALUES (new.rowid, new.text, new.label);
END;
