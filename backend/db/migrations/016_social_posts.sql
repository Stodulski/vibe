-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- SOCIAL POSTS — Vibe's own marketing posts, published to its Facebook Page and
-- its Instagram professional account.
--
-- None of these tables is tenant data: the rows describe Vibe itself, not a
-- complex, so neither carries complex_id or row-level security. Grants come
-- from the ALTER DEFAULT PRIVILEGES that 001_init.sql registered.
--
-- A post is written only by the content sync (PUT /api/v1/social/posts/{id}),
-- which authenticates with a GitHub Actions OIDC token. Its content is frozen
-- once any network has published it, or while a publish run holds it.
--
-- social_post_media stages uploads before their post exists: the sync sends
-- media first and the post second, so the upload cannot wait for a row that
-- the post upsert creates. The upsert copies the staged media it names into
-- social_posts.media, and that copy is what the publisher reads.
CREATE TABLE social_posts (
    id              TEXT PRIMARY KEY,
    publish_at      TIMESTAMPTZ NOT NULL,
    networks        TEXT[] NOT NULL,
    type            TEXT NOT NULL,
    caption         TEXT NOT NULL,
    alt_text        TEXT NOT NULL DEFAULT '',
    media           JSONB NOT NULL DEFAULT '[]'::jsonb,
    draft           BOOLEAN NOT NULL DEFAULT FALSE,
    status          TEXT NOT NULL DEFAULT 'scheduled',
    ig_media_id     TEXT,
    fb_post_id      TEXT,
    attempts        INTEGER NOT NULL DEFAULT 0,
    last_error      TEXT,
    next_attempt_at TIMESTAMPTZ,
    published_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT social_posts_id_format
        CHECK (id ~ '^[a-z0-9][a-z0-9-]{0,80}$'),
    CONSTRAINT social_posts_networks_known
        CHECK (cardinality(networks) > 0 AND networks <@ ARRAY['instagram', 'facebook']::TEXT[]),
    CONSTRAINT social_posts_type_check
        CHECK (type IN ('image', 'carousel', 'reel')),
    CONSTRAINT social_posts_status_check
        CHECK (status IN ('scheduled', 'publishing', 'published', 'partial', 'failed', 'cancelled')),
    CONSTRAINT social_posts_caption_length
        CHECK (char_length(caption) BETWEEN 1 AND 2200),
    CONSTRAINT social_posts_alt_text_length
        CHECK (char_length(alt_text) <= 1000),
    CONSTRAINT social_posts_attempts_check
        CHECK (attempts >= 0)
);

CREATE TRIGGER set_updated_at BEFORE UPDATE ON social_posts
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- The publisher's claim: live posts that are due, in publish order.
CREATE INDEX idx_social_posts_due ON social_posts (next_attempt_at)
    WHERE draft = FALSE AND status IN ('scheduled', 'partial', 'publishing');

CREATE TABLE social_post_media (
    post_id      TEXT NOT NULL,
    name         TEXT NOT NULL,
    url          TEXT NOT NULL,
    content_type TEXT NOT NULL,
    sha256       TEXT NOT NULL,
    size_bytes   BIGINT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (post_id, name),
    CONSTRAINT social_post_media_name_format
        CHECK (name ~ '^[A-Za-z0-9._-]{1,100}$'),
    CONSTRAINT social_post_media_size_positive
        CHECK (size_bytes > 0)
);

-- +goose Down

DROP TABLE IF EXISTS social_post_media;
DROP TABLE IF EXISTS social_posts;
