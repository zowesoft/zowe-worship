-- A schedule item usually takes the default theme for its kind, but one reading
-- sometimes needs a different look: a quiet Advent passage on the dark theme, a
-- celebratory psalm on the bright one. An empty id means "use the default".
ALTER TABLE service_items ADD COLUMN theme_id TEXT NOT NULL DEFAULT '';
