-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- SOCIAL STORY SEQUENCES — a story post is 1 to 6 files published as one story
-- each, in order, a minute apart. story_items holds one entry per file: its
-- position, its scheduled time, and the id each network gave it once it went
-- out. after_post_id names the feed post a story waits for on each network.
-- ============================================================================
ALTER TABLE social_posts
    ADD COLUMN story_items JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN after_post_id TEXT;

ALTER TABLE social_posts ADD CONSTRAINT social_posts_story_items_array
    CHECK (jsonb_typeof(story_items) = 'array');

ALTER TABLE social_posts ADD CONSTRAINT social_posts_after_format
    CHECK (after_post_id IS NULL OR after_post_id ~ '^[a-z0-9][a-z0-9-]{0,80}$');

-- Only a story waits for another post.
ALTER TABLE social_posts ADD CONSTRAINT social_posts_after_is_story
    CHECK (after_post_id IS NULL OR type = 'story');

-- +goose Down

SET LOCAL lock_timeout = '3s';

-- Rolling back drops the item state and the after gates. Any story that
-- already went out keeps its posts on the networks, but the database loses
-- which of its files went out, so roll back only before stories publish.
ALTER TABLE social_posts DROP CONSTRAINT IF EXISTS social_posts_after_is_story;
ALTER TABLE social_posts DROP CONSTRAINT IF EXISTS social_posts_after_format;
ALTER TABLE social_posts DROP CONSTRAINT IF EXISTS social_posts_story_items_array;

ALTER TABLE social_posts
    DROP COLUMN IF EXISTS after_post_id,
    DROP COLUMN IF EXISTS story_items;
