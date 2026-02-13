-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS links (
    slug TEXT PRIMARY KEY,
    url TEXT NOT NULL,
    notes TEXT,
    api_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    clicks BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_links_created_at_desc ON links (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_links_expires_at ON links (expires_at);

CREATE TABLE IF NOT EXISTS clicks_daily (
    slug TEXT NOT NULL,
    day DATE NOT NULL,
    count BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (slug, day)
);

CREATE INDEX IF NOT EXISTS idx_clicks_daily_slug_day_desc ON clicks_daily (slug, day DESC);

CREATE TABLE IF NOT EXISTS click_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type TEXT NOT NULL,
    slug TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    traceparent TEXT,
    tracestate TEXT,
    baggage TEXT,
    status TEXT NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processing_owner TEXT,
    processing_expires_at TIMESTAMPTZ,
    sent_at TIMESTAMPTZ,
    CONSTRAINT chk_click_outbox_status CHECK (status IN ('pending', 'processing', 'sent'))
);

CREATE INDEX IF NOT EXISTS idx_click_outbox_status_next_created
    ON click_outbox (status, next_attempt_at, created_at);
CREATE INDEX IF NOT EXISTS idx_click_outbox_status_processing_created
    ON click_outbox (status, processing_expires_at, created_at);
CREATE INDEX IF NOT EXISTS idx_click_outbox_created_at_desc ON click_outbox (created_at DESC);

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

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS click_processed_events;
DROP TABLE IF EXISTS click_outbox;
DROP TABLE IF EXISTS clicks_daily;
DROP TABLE IF EXISTS links;
-- +goose StatementEnd
