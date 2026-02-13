-- name: IncDailyClick :exec
INSERT INTO clicks_daily (slug, day, count)
VALUES ($1, $2, 1)
ON CONFLICT (slug, day)
DO UPDATE SET count = clicks_daily.count + 1;

-- name: GetDailyStatsByRange :many
SELECT slug, day, count
FROM clicks_daily
WHERE slug = $1
  AND day >= $2
  AND day <= $3
ORDER BY day ASC;

-- name: DeleteDailyStatsBySlug :exec
DELETE FROM clicks_daily WHERE slug = $1;
