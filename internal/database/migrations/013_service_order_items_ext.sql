-- Extend service_items with content for text/announcement slides, optional
-- operator notes, and estimated duration in seconds.
ALTER TABLE service_items ADD COLUMN content TEXT NOT NULL DEFAULT '';
ALTER TABLE service_items ADD COLUMN notes TEXT NOT NULL DEFAULT '';
ALTER TABLE service_items ADD COLUMN duration INTEGER NOT NULL DEFAULT 0;
