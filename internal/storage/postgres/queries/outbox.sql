-- name: EnqueueClickOutbox :one
INSERT INTO click_outbox (
    event_type,
    slug,
    occurred_at,
    traceparent,
    tracestate,
    baggage,
    status,
    attempts,
    next_attempt_at,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, 0, $8, $9, $9
)
RETURNING *;

-- name: ClaimNextOutboxEvent :one
WITH next_event AS (
    SELECT id
    FROM click_outbox
    WHERE (
            status = 'pending'
        AND next_attempt_at <= $1
    ) OR (
            status = 'processing'
        AND processing_expires_at <= $1
    )
    ORDER BY created_at ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
UPDATE click_outbox o
SET
    status = 'processing',
    processing_owner = $2,
    processing_expires_at = $3,
    updated_at = $1
FROM next_event
WHERE o.id = next_event.id
RETURNING o.*;

-- name: MarkOutboxSent :execrows
UPDATE click_outbox
SET
    status = 'sent',
    sent_at = $3,
    updated_at = $3,
    last_error = '',
    processing_owner = NULL,
    processing_expires_at = NULL
WHERE id = $1
  AND status = 'processing'
  AND processing_owner = $2;

-- name: MarkOutboxRetry :execrows
UPDATE click_outbox
SET
    status = 'pending',
    last_error = $3,
    next_attempt_at = $4,
    updated_at = $5,
    attempts = attempts + 1,
    processing_owner = NULL,
    processing_expires_at = NULL
WHERE id = $1
  AND status = 'processing'
  AND processing_owner = $2;
