-- A playlist is an ordered run of clips the operator plays back to back.
-- Membership is its own table, so reordering rewrites positions instead of
-- touching the clips. Foreign keys are on for every connection: deleting a
-- playlist drops its rows, and deleting a clip drops it from every playlist
-- it sat in, leaving no dangling ids behind.
CREATE TABLE media_playlists (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE media_playlist_items (
    playlist_id TEXT    NOT NULL REFERENCES media_playlists(id) ON DELETE CASCADE,
    position    INTEGER NOT NULL CHECK (position >= 0),
    media_id    TEXT    NOT NULL REFERENCES media_assets(id) ON DELETE CASCADE,
    PRIMARY KEY (playlist_id, position)
);

CREATE INDEX media_playlist_items_media ON media_playlist_items (media_id);
