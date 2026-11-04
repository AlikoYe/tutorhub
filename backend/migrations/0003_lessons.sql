-- +goose Up
-- Per-subject lesson length (NULL = the specialist's default) and the teacher's cancel rule.
ALTER TABLE specialist_subjects ADD COLUMN lesson_minutes int CHECK (lesson_minutes BETWEEN 30 AND 240);
ALTER TABLE specialists ADD COLUMN cancel_hours int NOT NULL DEFAULT 12 CHECK (cancel_hours BETWEEN 0 AND 72);

-- Weekly template: weekday follows Go/PostgreSQL DOW (0 = Sunday ... 6 = Saturday).
CREATE TABLE availability_rules (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    specialist_id bigint NOT NULL REFERENCES specialists (id) ON DELETE CASCADE,
    weekday       smallint NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    starts_at     time NOT NULL,
    ends_at       time NOT NULL,
    CHECK (ends_at > starts_at)  -- 24:00 is allowed as the end
);
CREATE INDEX availability_rules_specialist_idx ON availability_rules (specialist_id, weekday);

-- Days off (closed = true). Kept as a table so "extra working day" can be added later.
CREATE TABLE availability_exceptions (
    specialist_id bigint NOT NULL REFERENCES specialists (id) ON DELETE CASCADE,
    day           date NOT NULL,
    closed        boolean NOT NULL DEFAULT true,
    PRIMARY KEY (specialist_id, day)
);

CREATE TABLE lessons (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    specialist_id bigint NOT NULL REFERENCES specialists (id),
    student_id    bigint NOT NULL REFERENCES users (telegram_id),
    subject_id    bigint NOT NULL REFERENCES subjects (id),
    during        tstzrange NOT NULL CHECK (NOT isempty(during) AND lower_inc(during) AND NOT upper_inc(during)),
    status        text NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending', 'confirmed', 'declined', 'cancelled', 'expired', 'completed')),
    is_trial      boolean NOT NULL DEFAULT false,
    price_kzt     int NOT NULL DEFAULT 0 CHECK (price_kzt >= 0),
    note          text NOT NULL DEFAULT '',
    expires_at    timestamptz,
    decided_at    timestamptz,
    cancelled_by  text CHECK (cancelled_by IN ('student', 'teacher')),
    cancel_reason text NOT NULL DEFAULT '',
    reminder_sent boolean NOT NULL DEFAULT false,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CHECK (status <> 'pending' OR expires_at IS NOT NULL),
    CONSTRAINT lessons_no_specialist_overlap EXCLUDE USING gist (specialist_id WITH =, during WITH &&)
        WHERE (status IN ('pending', 'confirmed')),
    CONSTRAINT lessons_no_student_overlap EXCLUDE USING gist (student_id WITH =, during WITH &&)
        WHERE (status IN ('pending', 'confirmed'))
);
CREATE INDEX lessons_student_idx ON lessons (student_id, lower(during));
CREATE INDEX lessons_specialist_idx ON lessons (specialist_id, lower(during));
CREATE INDEX lessons_pending_idx ON lessons (expires_at) WHERE status = 'pending';

-- The teacher's private notes about a student.
CREATE TABLE student_notes (
    specialist_id bigint NOT NULL REFERENCES specialists (id) ON DELETE CASCADE,
    student_id    bigint NOT NULL REFERENCES users (telegram_id),
    note          text NOT NULL DEFAULT '',
    updated_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (specialist_id, student_id)
);

-- +goose Down
DROP TABLE student_notes;
DROP TABLE lessons;
DROP TABLE availability_exceptions;
DROP TABLE availability_rules;
ALTER TABLE specialists DROP COLUMN cancel_hours;
ALTER TABLE specialist_subjects DROP COLUMN lesson_minutes;
