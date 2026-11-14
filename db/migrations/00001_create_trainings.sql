-- +goose Up
CREATE TABLE trainings (
    id         BIGSERIAL PRIMARY KEY,
    date       DATE        NOT NULL,
    title      TEXT        NOT NULL,
    notes      TEXT        NOT NULL DEFAULT '',
    blocks     JSONB       NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX trainings_date_idx ON trainings (date);

-- +goose Down
DROP TABLE trainings;
