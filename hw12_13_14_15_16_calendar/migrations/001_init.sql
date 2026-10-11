-- +goose Up

CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE events (
                        id UUID PRIMARY KEY,
                        user_id BIGINT NOT NULL,
                        title TEXT NOT NULL,
                        description TEXT NOT NULL DEFAULT '',
                        starts_at TIMESTAMPTZ NOT NULL,
                        ends_at TIMESTAMPTZ NOT NULL,
                        notify_before_ns BIGINT NOT NULL DEFAULT 0,

                        CONSTRAINT valid_event_interval
                            CHECK (ends_at > starts_at),

                        CONSTRAINT valid_notify_before
                            CHECK (notify_before_ns >= 0),

                        CONSTRAINT events_no_overlap
                            EXCLUDE USING gist (
            user_id WITH =,
            tstzrange(starts_at, ends_at, '[)') WITH &&
        )
);

CREATE INDEX events_user_starts_at_idx
    ON events (user_id, starts_at);

-- +goose Down

DROP TABLE IF EXISTS events;