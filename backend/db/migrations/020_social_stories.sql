-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- SOCIAL STORIES — a story is one image or one MP4 shown for 24 hours. It goes
-- to Instagram or Facebook, never Threads. It has no caption: the Instagram
-- story container and the Page story endpoints take none, so a story may be
-- stored with an empty caption. Every other type still needs one.
-- ============================================================================
ALTER TABLE social_posts DROP CONSTRAINT social_posts_type_check;
ALTER TABLE social_posts ADD CONSTRAINT social_posts_type_check
    CHECK (type IN ('image', 'carousel', 'reel', 'text', 'story'));

ALTER TABLE social_posts DROP CONSTRAINT social_posts_caption_length;
ALTER TABLE social_posts ADD CONSTRAINT social_posts_caption_length
    CHECK (char_length(caption) <= 2200 AND (type = 'story' OR char_length(caption) >= 1));

ALTER TABLE social_posts ADD CONSTRAINT social_posts_story_not_threads
    CHECK (type <> 'story' OR NOT (networks && ARRAY['threads']::TEXT[]));

-- +goose Down

SET LOCAL lock_timeout = '3s';

-- The down step refuses while a story row exists: each re-added check is
-- validated against every row, so a story cannot be silently reshaped into
-- another type or given the caption it never had.
ALTER TABLE social_posts DROP CONSTRAINT IF EXISTS social_posts_story_not_threads;

ALTER TABLE social_posts DROP CONSTRAINT social_posts_caption_length;
ALTER TABLE social_posts ADD CONSTRAINT social_posts_caption_length
    CHECK (char_length(caption) BETWEEN 1 AND 2200);

ALTER TABLE social_posts DROP CONSTRAINT social_posts_type_check;
ALTER TABLE social_posts ADD CONSTRAINT social_posts_type_check
    CHECK (type IN ('image', 'carousel', 'reel', 'text'));
