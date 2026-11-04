-- +goose Up
-- One review per student per teacher, only after a completed lesson; editable.
CREATE TABLE reviews (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    specialist_id bigint NOT NULL REFERENCES specialists (id) ON DELETE CASCADE,
    student_id    bigint NOT NULL REFERENCES users (telegram_id),
    lesson_id     bigint NOT NULL REFERENCES lessons (id),
    rating        smallint NOT NULL CHECK (rating BETWEEN 1 AND 5),
    text          text NOT NULL DEFAULT '',
    status        text NOT NULL DEFAULT 'published' CHECK (status IN ('published', 'hidden')),
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (specialist_id, student_id)
);
CREATE INDEX reviews_specialist_idx ON reviews (specialist_id, status, created_at);

CREATE TABLE reports (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    specialist_id bigint NOT NULL REFERENCES specialists (id) ON DELETE CASCADE,
    reporter_id   bigint NOT NULL REFERENCES users (telegram_id),
    reason        text NOT NULL CHECK (reason IN ('fake', 'inappropriate', 'spam', 'no_show', 'other')),
    text          text NOT NULL DEFAULT '',
    status        text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'resolved', 'dismissed')),
    created_at    timestamptz NOT NULL DEFAULT now(),
    resolved_at   timestamptz,
    resolved_by   bigint REFERENCES users (telegram_id)
);
CREATE INDEX reports_status_idx ON reports (status, created_at);

ALTER TABLE users ADD COLUMN blocked boolean NOT NULL DEFAULT false;

-- Verified badge flow: the teacher uploads documents (admins only can read them) and asks.
ALTER TABLE specialists
    ADD COLUMN verification_status text NOT NULL DEFAULT 'none'
        CHECK (verification_status IN ('none', 'requested', 'verified', 'rejected')),
    ADD COLUMN verification_note text NOT NULL DEFAULT '',
    ADD COLUMN block_reason text NOT NULL DEFAULT '';
UPDATE specialists SET verification_status = 'verified' WHERE verified;

CREATE TABLE verification_documents (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    specialist_id bigint NOT NULL REFERENCES specialists (id) ON DELETE CASCADE,
    file_name     text NOT NULL UNIQUE,
    original_name text NOT NULL,
    content_type  text NOT NULL,
    size_bytes    int NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE verification_documents;
ALTER TABLE specialists DROP COLUMN block_reason, DROP COLUMN verification_note, DROP COLUMN verification_status;
ALTER TABLE users DROP COLUMN blocked;
DROP TABLE reports;
DROP TABLE reviews;
