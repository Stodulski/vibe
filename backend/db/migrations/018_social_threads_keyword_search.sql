-- +goose Up

SET LOCAL lock_timeout = '3s';

-- Whether the consent that produced this token asked for threads_keyword_search.
-- Connections made before that scope existed read FALSE: their token was never
-- granted the scope, so keyword search needs the owner to authorize again.
ALTER TABLE social_threads_connection
    ADD COLUMN keyword_search_requested BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down

ALTER TABLE social_threads_connection DROP COLUMN IF EXISTS keyword_search_requested;
