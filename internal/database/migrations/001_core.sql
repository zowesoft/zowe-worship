CREATE TABLE bible_translations (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    abbreviation TEXT NOT NULL,
    language     TEXT NOT NULL,
    version      TEXT NOT NULL,
    filename     TEXT NOT NULL UNIQUE,
    verse_count  INTEGER NOT NULL CHECK (verse_count > 0),
    checksum     TEXT NOT NULL,
    installed_at TEXT NOT NULL
);
