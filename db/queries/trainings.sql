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
