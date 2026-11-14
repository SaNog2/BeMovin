-- name: CreateComment :one
INSERT INTO comments (training_id, author, body)
SELECT t.id, sqlc.arg(author), sqlc.arg(body)
FROM trainings t
WHERE t.id = sqlc.arg(training_id)
RETURNING *;

-- name: ListComments :many
SELECT * FROM comments
WHERE training_id = $1
ORDER BY created_at, id;

-- name: DeleteComment :execrows
-- Deletes a comment if it was written by author, or by anyone when any_author is set (admin).
DELETE FROM comments
WHERE id = sqlc.arg(id)
  AND training_id = sqlc.arg(training_id)
  AND (author = sqlc.arg(author) OR sqlc.arg(any_author)::boolean);
