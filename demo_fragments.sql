-- fragment: user_fields
SELECT id, name, email, created_at FROM users

-- fragment: active_users
WHERE status = 'active' AND deleted_at IS NULL

-- name: GetActiveUser :one
{{fragment:user_fields}}
{{fragment:active_users}}
AND id = $1
LIMIT 1;

-- name: ListActiveUsers :many
{{fragment:user_fields}}
{{fragment:active_users}}
ORDER BY created_at DESC;