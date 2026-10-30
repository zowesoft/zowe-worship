-- The library owns a copy of every imported file, so `filename` is the name
-- inside the media directory and `source_path` is only provenance.
CREATE TABLE media_assets (
    id            TEXT PRIMARY KEY,
    kind          TEXT NOT NULL CHECK (kind IN ('image', 'video', 'audio')),
    title         TEXT NOT NULL,
    filename      TEXT NOT NULL UNIQUE,
    source_path   TEXT NOT NULL,
    checksum      TEXT NOT NULL,
    size_bytes    INTEGER NOT NULL CHECK (size_bytes > 0),
    duration_ms   INTEGER NOT NULL DEFAULT 0,
    width         INTEGER NOT NULL DEFAULT 0,
    height        INTEGER NOT NULL DEFAULT 0,
    has_thumbnail INTEGER NOT NULL DEFAULT 0,
    created_at    TEXT NOT NULL
);

-- Duplicate detection runs on import, and it is the one query the library makes
-- by something other than title.
CREATE INDEX media_assets_checksum ON media_assets (checksum);
