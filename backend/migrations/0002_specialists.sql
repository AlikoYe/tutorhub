-- +goose Up
CREATE TABLE specialists (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id          bigint NOT NULL UNIQUE REFERENCES users (telegram_id),
    kind             text NOT NULL DEFAULT 'teacher' CHECK (kind IN ('teacher', 'consultant')),
    status           text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'pending', 'live', 'hidden', 'blocked')),
    display_name     text NOT NULL DEFAULT '',
    photo_url        text,
    bio              text NOT NULL DEFAULT '',
    education        text NOT NULL DEFAULT '',
    experience_years int NOT NULL DEFAULT 0 CHECK (experience_years BETWEEN 0 AND 70),
    verified         boolean NOT NULL DEFAULT false,
    city             text NOT NULL DEFAULT 'almaty',
    district         text NOT NULL DEFAULT '',
    address          text NOT NULL DEFAULT '',
    lat              double precision CHECK (lat BETWEEN -90 AND 90),
    lng              double precision CHECK (lng BETWEEN -180 AND 180),
    formats          text[] NOT NULL DEFAULT '{}' CHECK (formats <@ ARRAY['online', 'teacher_place', 'student_home']),
    languages        text[] NOT NULL DEFAULT '{}' CHECK (languages <@ ARRAY['kk', 'ru', 'en']),
    trial_kind       text NOT NULL DEFAULT 'none' CHECK (trial_kind IN ('none', 'free', 'paid')),
    trial_price      int CHECK (trial_price >= 0),
    lesson_minutes   int NOT NULL DEFAULT 60 CHECK (lesson_minutes BETWEEN 30 AND 240),
    rejection_reason text,
    submitted_at     timestamptz,
    approved_at      timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    CHECK (trial_kind = 'paid' OR trial_price IS NULL)
);
CREATE INDEX specialists_status_idx ON specialists (status, city, district);
CREATE INDEX specialists_name_trgm_idx ON specialists USING gin (display_name gin_trgm_ops);

CREATE TABLE specialist_subjects (
    specialist_id bigint NOT NULL REFERENCES specialists (id) ON DELETE CASCADE,
    subject_id    bigint NOT NULL REFERENCES subjects (id),
    levels        text[] NOT NULL DEFAULT '{}'
                  CHECK (levels <@ ARRAY['primary', 'middle', 'high', 'ent', 'university', 'adult']),
    price_kzt     int NOT NULL CHECK (price_kzt BETWEEN 0 AND 1000000),
    PRIMARY KEY (specialist_id, subject_id)
);
CREATE INDEX specialist_subjects_subject_idx ON specialist_subjects (subject_id, price_kzt);

-- Paid visibility: a specialist is listed only while one of these is running (see package visibility).
CREATE TABLE subscriptions (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    specialist_id    bigint NOT NULL REFERENCES specialists (id) ON DELETE CASCADE,
    plan             text NOT NULL CHECK (plan IN ('founding', 'trial', 'basic', 'featured')),
    starts_at        timestamptz NOT NULL,
    ends_at          timestamptz NOT NULL,
    note             text NOT NULL DEFAULT '',
    created_by_admin bigint REFERENCES users (telegram_id),
    created_at       timestamptz NOT NULL DEFAULT now(),
    CHECK (ends_at > starts_at)
);
CREATE INDEX subscriptions_specialist_idx ON subscriptions (specialist_id, ends_at);

-- +goose Down
DROP TABLE subscriptions;
DROP TABLE specialist_subjects;
DROP TABLE specialists;
