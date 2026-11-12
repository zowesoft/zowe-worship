-- A service keeps its own working history: what went on the wall while it
-- ran, and what the operator decided about the speech they heard in it.
-- Without this, one service's projections and decisions were listed under
-- every other service on the machine.
--
-- Rows written before this migration belong to no service and stay that way:
-- they cannot honestly be attributed to one.
ALTER TABLE presentation_history ADD COLUMN service_id TEXT REFERENCES services(id) ON DELETE CASCADE;
ALTER TABLE detection_history ADD COLUMN service_id TEXT REFERENCES services(id) ON DELETE CASCADE;

CREATE INDEX presentation_history_service ON presentation_history (service_id, projected_at DESC);
CREATE INDEX detection_history_service ON detection_history (service_id, created_at DESC);
