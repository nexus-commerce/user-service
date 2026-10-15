ALTER TABLE outbox ADD COLUMN event_id UUID NOT NULL DEFAULT gen_random_uuid();
ALTER TABLE outbox ALTER COLUMN event_id DROP DEFAULT;
ALTER TABLE outbox ADD CONSTRAINT outbox_event_id_key UNIQUE (event_id);
