-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- SOCIAL THREADS — Vibe's Threads account joins Facebook and Instagram as a
-- network for its own posts, and a text-only post type goes to Threads alone.
--
-- The reel cover is a JPEG uploaded with the post's media. It is stored on the
-- post, not in social_post_media's role list, because a reel's media count is
-- one video and the cover is not part of it.
--
-- social_threads_connection holds the one Threads account. Its access token is
-- sealed with the credential keyring (MP_CREDENTIAL_KEYS) before it is written,
-- so a database dump never holds a usable token. The table is not tenant data.
ALTER TABLE social_posts DROP CONSTRAINT social_posts_networks_known;
ALTER TABLE social_posts ADD CONSTRAINT social_posts_networks_known
    CHECK (cardinality(networks) > 0 AND networks <@ ARRAY['instagram', 'facebook', 'threads']::TEXT[]);

ALTER TABLE social_posts DROP CONSTRAINT social_posts_type_check;
ALTER TABLE social_posts ADD CONSTRAINT social_posts_type_check
    CHECK (type IN ('image', 'carousel', 'reel', 'text'));

-- A text post is Threads-only: its networks are exactly ['threads'].
ALTER TABLE social_posts ADD CONSTRAINT social_posts_text_is_threads_only
    CHECK (type <> 'text' OR networks = ARRAY['threads']::TEXT[]);

ALTER TABLE social_posts
    ADD COLUMN threads_media_id TEXT,
    ADD COLUMN threads_text     TEXT,
    ADD COLUMN threads_topic    TEXT,
    ADD COLUMN cover            JSONB,
    ADD CONSTRAINT social_posts_threads_text_length
        CHECK (threads_text IS NULL OR char_length(threads_text) BETWEEN 1 AND 500),
    ADD CONSTRAINT social_posts_threads_topic_length
        CHECK (threads_topic IS NULL OR char_length(threads_topic) BETWEEN 1 AND 50);

CREATE TABLE social_threads_connection (
    id           SMALLINT PRIMARY KEY DEFAULT 1,
    user_id      TEXT NOT NULL,
    username     TEXT NOT NULL,
    access_token TEXT NOT NULL,
    connected_at TIMESTAMPTZ NOT NULL,
    refreshed_at TIMESTAMPTZ NOT NULL,
    expires_at   TIMESTAMPTZ NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT social_threads_connection_singleton CHECK (id = 1)
);

CREATE TRIGGER set_updated_at BEFORE UPDATE ON social_threads_connection
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- +goose Down

DROP TABLE IF EXISTS social_threads_connection;

ALTER TABLE social_posts
    DROP CONSTRAINT IF EXISTS social_posts_threads_topic_length,
    DROP CONSTRAINT IF EXISTS social_posts_threads_text_length,
    DROP COLUMN IF EXISTS cover,
    DROP COLUMN IF EXISTS threads_topic,
    DROP COLUMN IF EXISTS threads_text,
    DROP COLUMN IF EXISTS threads_media_id;

ALTER TABLE social_posts DROP CONSTRAINT IF EXISTS social_posts_text_is_threads_only;

ALTER TABLE social_posts DROP CONSTRAINT social_posts_type_check;
ALTER TABLE social_posts ADD CONSTRAINT social_posts_type_check
    CHECK (type IN ('image', 'carousel', 'reel'));

ALTER TABLE social_posts DROP CONSTRAINT social_posts_networks_known;
ALTER TABLE social_posts ADD CONSTRAINT social_posts_networks_known
    CHECK (cardinality(networks) > 0 AND networks <@ ARRAY['instagram', 'facebook']::TEXT[]);
