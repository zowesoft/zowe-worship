-- How an image is framed for output. `aspect` is the box the operator chose for
-- this artwork and `fit` says whether the picture shows all of itself inside
-- that box or fills it and loses its edges.
ALTER TABLE media_assets ADD COLUMN aspect TEXT NOT NULL DEFAULT 'auto';
ALTER TABLE media_assets ADD COLUMN fit TEXT NOT NULL DEFAULT 'contain';
