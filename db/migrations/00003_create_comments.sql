-- +goose Up
CREATE TABLE comments (
    id          BIGSERIAL PRIMARY KEY,
    training_id BIGINT      NOT NULL REFERENCES trainings (id) ON DELETE CASCADE,
    author      TEXT        NOT NULL,
    body        TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX comments_training_id_idx ON comments (training_id, created_at);

-- +goose Down
DROP TABLE comments;
