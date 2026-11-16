-- +goose Up
ALTER TABLE events
    ADD COLUMN version BIGINT NOT NULL DEFAULT 1;

CREATE TABLE notification_outbox (
     id UUID PRIMARY KEY,

     event_id UUID NOT NULL
         REFERENCES events(id) ON DELETE CASCADE,

     claim_id UUID,

     event_version BIGINT NOT NULL,

     payload JSONB NOT NULL,

     status TEXT NOT NULL DEFAULT 'pending'
         CHECK (status IN ('pending', 'processing', 'sent', 'cancelled')),

     claimed_until TIMESTAMPTZ,

     created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
     sent_at TIMESTAMPTZ,

     UNIQUE (event_id, event_version)
);

CREATE INDEX notification_outbox_pending_idx
    ON notification_outbox (created_at, id)
    WHERE status = 'pending';

CREATE INDEX notification_outbox_processing_idx
    ON notification_outbox (claimed_until)
    WHERE status = 'processing';

-- +goose Down
DROP TABLE notification_outbox;

ALTER TABLE events
DROP COLUMN version;
