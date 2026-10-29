-- Application settings live beside the catalog so the choice of current service
-- survives a restart without a per-window flag.
CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Whatever the app was already showing stays current once switching exists.
INSERT INTO settings (key, value)
SELECT 'current_service_id', id
FROM services
ORDER BY created_at DESC, rowid DESC
LIMIT 1;
