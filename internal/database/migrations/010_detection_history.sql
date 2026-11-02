-- Records spoken scripture detection decisions, candidates and confidence scores.
CREATE TABLE detection_history (
    id         TEXT PRIMARY KEY,
    transcript TEXT NOT NULL,
    reference  TEXT NOT NULL,
    confidence REAL NOT NULL,
    decision   TEXT NOT NULL CHECK (decision IN ('auto_projected', 'approved', 'dismissed', 'ignored')),
    created_at TEXT NOT NULL
);

CREATE INDEX detection_history_created_at ON detection_history (created_at DESC);
