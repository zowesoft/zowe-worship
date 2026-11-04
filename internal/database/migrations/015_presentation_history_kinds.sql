-- 009 pinned `kind` to four values, written before the schedule grew its item
-- kinds. Projections now reach the room as text, announcement, blank and the
-- three media kinds as well, so that CHECK would reject perfectly good rows.
-- The vocabulary belongs with the slide it describes, not in the schema, so the
-- constraint is dropped and Go owns the values.
--
-- SQLite cannot alter a CHECK, so the table is rebuilt. The index is recreated
-- after the rename because dropping the old table takes its indexes with it.
CREATE TABLE presentation_history_new (
    id           TEXT PRIMARY KEY,
    kind         TEXT NOT NULL,
    title        TEXT NOT NULL,
    label        TEXT NOT NULL DEFAULT '',
    source       TEXT NOT NULL DEFAULT 'manual',
    payload      TEXT NOT NULL DEFAULT '',
    projected_at TEXT NOT NULL
);

INSERT INTO presentation_history_new (id, kind, title, label, source, payload, projected_at)
    SELECT id, kind, title, label, source, payload, projected_at FROM presentation_history;

DROP TABLE presentation_history;

ALTER TABLE presentation_history_new RENAME TO presentation_history;

CREATE INDEX presentation_history_projected_at ON presentation_history (projected_at DESC);
