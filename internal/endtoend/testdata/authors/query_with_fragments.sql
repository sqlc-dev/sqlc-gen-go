-- fragment: author_select
SELECT id, name, bio FROM authors

-- fragment: active_filter
WHERE deleted_at IS NULL

-- name: GetActiveAuthor :one
{{fragment:author_select}}
{{fragment:active_filter}}
AND id = $1
LIMIT 1;

-- name: ListActiveAuthors :many
{{fragment:author_select}}
{{fragment:active_filter}}
ORDER BY name;

-- name: CreateAuthor :one
INSERT INTO authors (
  name, bio
) VALUES (
  $1, $2
)
RETURNING *;