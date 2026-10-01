-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS shortener;

CREATE TABLE shortener.links
(
    id           UUID PRIMARY KEY     DEFAULT gen_random_uuid(),
    code         VARCHAR(32) NOT NULL,
    original_url TEXT        NOT NULL,
    clicks       BIGINT      NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT links_code_key UNIQUE (code)
);

CREATE INDEX links_created_at_idx ON shortener.links (created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS shortener.links;
DROP SCHEMA IF EXISTS shortener;
-- +goose StatementEnd
