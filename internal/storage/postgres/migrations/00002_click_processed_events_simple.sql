-- +goose Up
-- +goose StatementBegin
DROP TABLE IF EXISTS click_processed_events;

CREATE TABLE IF NOT EXISTS click_processed_events (
    event_id TEXT PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS click_processed_events;

CREATE TABLE IF NOT EXISTS click_processed_events (
    event_id TEXT PRIMARY KEY,
    slug TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    state TEXT NOT NULL,
    owner TEXT,
    lease_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    done_at TIMESTAMPTZ,
    CONSTRAINT chk_click_processed_state CHECK (state IN ('processing', 'done'))
);

CREATE INDEX IF NOT EXISTS idx_click_processed_state_lease
    ON click_processed_events (state, lease_until);
-- +goose StatementEnd
