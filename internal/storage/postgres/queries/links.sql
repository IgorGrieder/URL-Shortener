-- name: CreateLink :one
INSERT INTO links (
    slug,
    url,
    notes,
    api_key,
    created_at,
    expires_at,
    clicks
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: GetLinkBySlug :one
SELECT * FROM links WHERE slug = $1;

-- name: GetActiveLinkBySlug :one
SELECT * FROM links
WHERE slug = $1
  AND (expires_at IS NULL OR expires_at >= $2);

-- name: GetActiveLinkBySlugAndIncClick :one
UPDATE links
SET clicks = clicks + 1
WHERE slug = $1
  AND (expires_at IS NULL OR expires_at >= $2)
RETURNING *;

-- name: DeleteLinkBySlug :execrows
DELETE FROM links WHERE slug = $1;
