-- name: InsertProcessedEventOnce :execrows
INSERT INTO click_processed_events (
    event_id,
    processed_at
) VALUES (
    $1, $2
)
ON CONFLICT (event_id) DO NOTHING;
