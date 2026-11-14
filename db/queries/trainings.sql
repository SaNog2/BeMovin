-- name: CreateTraining :one
INSERT INTO trainings (date, title, notes, blocks)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetTraining :one
SELECT * FROM trainings
WHERE id = $1;

-- name: ListTrainings :many
SELECT * FROM trainings
ORDER BY date DESC, id DESC;

-- name: ListTrainingsBetween :many
SELECT * FROM trainings
WHERE date >= sqlc.arg(start_date) AND date < sqlc.arg(end_date)
ORDER BY date, id;

-- name: CompleteTraining :execrows
UPDATE trainings
SET completed_at    = COALESCE(completed_at, now()),
    feeling         = sqlc.narg(feeling),
    completion_note = sqlc.arg(completion_note)
WHERE id = sqlc.arg(id);

-- name: UncompleteTraining :execrows
UPDATE trainings
SET completed_at = NULL, feeling = NULL, completion_note = ''
WHERE id = $1;
