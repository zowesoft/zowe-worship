-- Records projection actions in local presentation history, so operators can
-- review what was on screen during a service and restore past outputs.
CREATE TABLE presentation_history (
    id           TEXT PRIMARY KEY,
    kind         TEXT NOT NULL CHECK (kind IN ('scripture', 'song', 'media', 'clear')),
    title        TEXT NOT NULL,
    label        TEXT NOT NULL DEFAULT '',
    source       TEXT NOT NULL DEFAULT 'manual',
    payload      TEXT NOT NULL DEFAULT '',
    projected_at TEXT NOT NULL
);

CREATE INDEX presentation_history_projected_at ON presentation_history (projected_at DESC);
