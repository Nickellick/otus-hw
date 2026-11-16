-- +goose Up

ALTER TABLE events
    ADD COLUMN notification_sent_at TIMESTAMPTZ;

CREATE INDEX events_pending_notifications_idx
    ON events (starts_at)
    WHERE notify_before_ns > 0
      AND notification_sent_at IS NULL;

-- +goose Down

DROP INDEX IF EXISTS events_pending_notifications_idx;

ALTER TABLE events
DROP COLUMN notification_sent_at;
