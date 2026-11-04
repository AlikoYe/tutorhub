-- +goose Up
-- Expiry notifications are sent once per subscription (worker).
ALTER TABLE subscriptions
    ADD COLUMN reminded_7d   boolean NOT NULL DEFAULT false,
    ADD COLUMN reminded_1d   boolean NOT NULL DEFAULT false,
    ADD COLUMN expiry_notified boolean NOT NULL DEFAULT false;

-- Funnel events for teacher statistics (append-only).
CREATE TABLE profile_events (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    specialist_id bigint NOT NULL REFERENCES specialists (id) ON DELETE CASCADE,
    kind          text NOT NULL CHECK (kind IN ('view', 'search_impression', 'contact', 'request')),
    user_id       bigint REFERENCES users (telegram_id),
    at            timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX profile_events_specialist_idx ON profile_events (specialist_id, at);
CREATE INDEX profile_events_kind_at_idx ON profile_events (kind, at);

-- One row per catalogue search (first page), for the admin dashboard and unmet demand.
CREATE TABLE search_log (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    bigint REFERENCES users (telegram_id),
    subject_id bigint REFERENCES subjects (id),
    district   text NOT NULL DEFAULT '',
    query      text NOT NULL DEFAULT '',
    results    int NOT NULL,
    at         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX search_log_at_idx ON search_log (at);

CREATE TABLE favourites (
    user_id       bigint NOT NULL REFERENCES users (telegram_id),
    specialist_id bigint NOT NULL REFERENCES specialists (id) ON DELETE CASCADE,
    created_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, specialist_id)
);

-- +goose Down
DROP TABLE favourites;
DROP TABLE search_log;
DROP TABLE profile_events;
ALTER TABLE subscriptions DROP COLUMN expiry_notified, DROP COLUMN reminded_1d, DROP COLUMN reminded_7d;
