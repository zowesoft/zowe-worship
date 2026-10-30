-- A media item references a library asset rather than carrying its own copy, so
-- the same image can appear in several services without duplicating the file.
ALTER TABLE service_items ADD COLUMN media_id TEXT NOT NULL DEFAULT '';
