-- name: GetFeedsByURLS :one
SELECT * FROM feeds
WHERE url = $1;