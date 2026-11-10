-- What a translation may be used under, and the line that must be shown with
-- it. Recorded here so the UI can always answer "whose words are these?".
ALTER TABLE bible_translations ADD COLUMN license TEXT NOT NULL DEFAULT '';
ALTER TABLE bible_translations ADD COLUMN attribution TEXT NOT NULL DEFAULT '';
