-- +goose Up
ALTER TABLE trainings
    ADD COLUMN completed_at    TIMESTAMPTZ,
    ADD COLUMN feeling         SMALLINT CHECK (feeling BETWEEN 1 AND 5),
    ADD COLUMN completion_note TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE trainings
    DROP COLUMN completed_at,
    DROP COLUMN feeling,
    DROP COLUMN completion_note;
