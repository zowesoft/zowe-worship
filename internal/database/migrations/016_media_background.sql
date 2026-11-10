-- The picture an audio or video asset shows behind itself while it plays, as a
-- library id so a moved file cannot break a service.
ALTER TABLE media_assets ADD COLUMN background_id TEXT NOT NULL DEFAULT '';
