// Package store persists marketing posts and their staged uploads in
// social_posts and social_post_media, and the Threads connection in
// social_threads_connection. The post tables are not tenant data, so their
// queries carry no tenant scope.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/social"
)

// Posts is the social post store.
type Posts struct {
	DB *data.DB
}

// NewPosts returns a Posts over db.
func NewPosts(db *data.DB) *Posts {
	return &Posts{DB: db}
}

const postColumns = `id, publish_at, networks, type, caption, alt_text, media, draft, status,
	ig_media_id, fb_post_id, attempts, last_error, next_attempt_at, published_at,
	threads_media_id, threads_text, threads_topic, cover`

// upsertSQL writes a post's content. The WHERE clause is the freeze: a row
// that a network has published, or that a run holds, is left alone and the
// statement returns no row. A cancelled or failed post is reset to scheduled.
const upsertSQL = `
INSERT INTO social_posts
	(id, publish_at, networks, type, caption, alt_text, media, draft, status, attempts, next_attempt_at,
	 threads_text, threads_topic, cover)
VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, 'scheduled', 0, $2, $9, $10, $11::jsonb)
ON CONFLICT (id) DO UPDATE SET
	publish_at      = EXCLUDED.publish_at,
	networks        = EXCLUDED.networks,
	type            = EXCLUDED.type,
	caption         = EXCLUDED.caption,
	alt_text        = EXCLUDED.alt_text,
	media           = EXCLUDED.media,
	draft           = EXCLUDED.draft,
	threads_text    = EXCLUDED.threads_text,
	threads_topic   = EXCLUDED.threads_topic,
	cover           = EXCLUDED.cover,
	status          = 'scheduled',
	attempts        = 0,
	last_error      = NULL,
	next_attempt_at = EXCLUDED.publish_at
WHERE social_posts.ig_media_id IS NULL
	AND social_posts.fb_post_id IS NULL
	AND social_posts.threads_media_id IS NULL
	AND social_posts.status <> 'publishing'
RETURNING id`

// Upsert stores a post's content. It returns social.ErrFrozen when the post
// may no longer change.
func (s *Posts) Upsert(ctx context.Context, p social.Post) error {
	media, err := json.Marshal(nonNilMedia(p.Media))
	if err != nil {
		return fmt.Errorf("social: encode media: %w", err)
	}
	var cover any
	if p.Cover != nil {
		raw, err := json.Marshal(p.Cover)
		if err != nil {
			return fmt.Errorf("social: encode cover: %w", err)
		}
		cover = string(raw)
	}
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	var id string
	err = s.DB.QueryRow(ctx, upsertSQL,
		p.ID, p.PublishAt, networkStrings(p.Networks), string(p.Kind), p.Caption, p.AltText, string(media), p.Draft,
		nullable(p.ThreadsText), nullable(p.ThreadsTopic), cover,
	).Scan(&id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return social.ErrFrozen
	case err != nil:
		return fmt.Errorf("social: upsert post: %w", err)
	default:
		return nil
	}
}

// StageMedia records an upload under its post so a later upsert can name it.
func (s *Posts) StageMedia(ctx context.Context, postID string, m social.Media) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	_, err := s.DB.Exec(ctx, `
		INSERT INTO social_post_media (post_id, name, url, content_type, sha256, size_bytes)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (post_id, name) DO UPDATE SET
			url          = EXCLUDED.url,
			content_type = EXCLUDED.content_type,
			sha256       = EXCLUDED.sha256,
			size_bytes   = EXCLUDED.size_bytes,
			created_at   = NOW()`,
		postID, m.Name, m.URL, m.ContentType, m.SHA256, m.SizeBytes)
	if err != nil {
		return fmt.Errorf("social: stage media: %w", err)
	}
	return nil
}

// StagedMedia returns the staged uploads of postID among names.
func (s *Posts) StagedMedia(ctx context.Context, postID string, names []string) ([]social.Media, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := s.DB.Query(ctx, `
		SELECT name, url, content_type, sha256, size_bytes
		FROM social_post_media
		WHERE post_id = $1 AND name = ANY($2::text[])`,
		postID, nonNilStrings(names))
	if err != nil {
		return nil, fmt.Errorf("social: read staged media: %w", err)
	}
	defer rows.Close()

	var out []social.Media
	for rows.Next() {
		var m social.Media
		if err := rows.Scan(&m.Name, &m.URL, &m.ContentType, &m.SHA256, &m.SizeBytes); err != nil {
			return nil, fmt.Errorf("social: scan staged media: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("social: read staged media: %w", err)
	}
	return out, nil
}

// IsFrozen reports whether post id exists and its content may no longer change.
func (s *Posts) IsFrozen(ctx context.Context, id string) (bool, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	var frozen bool
	err := s.DB.QueryRow(ctx, `
		SELECT ig_media_id IS NOT NULL OR fb_post_id IS NOT NULL OR threads_media_id IS NOT NULL
			OR status = 'publishing'
		FROM social_posts WHERE id = $1`, id).Scan(&frozen)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("social: read post state: %w", err)
	default:
		return frozen, nil
	}
}

// CancelUnpublished cancels every scheduled post that no network has published
// and whose id is not in keep. It returns the ids it cancelled, sorted.
func (s *Posts) CancelUnpublished(ctx context.Context, keep []string) ([]string, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := s.DB.Query(ctx, `
		UPDATE social_posts SET status = 'cancelled', next_attempt_at = NULL
		WHERE status = 'scheduled'
			AND ig_media_id IS NULL
			AND fb_post_id IS NULL
			AND threads_media_id IS NULL
			AND NOT (id = ANY($1::text[]))
		RETURNING id`, nonNilStrings(keep))
	if err != nil {
		return nil, fmt.Errorf("social: cancel unpublished posts: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("social: scan cancelled post: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("social: cancel unpublished posts: %w", err)
	}
	slices.Sort(ids)
	return ids, nil
}

// ClaimDue marks up to limit live, due posts as publishing until now+lease and
// returns them. SKIP LOCKED keeps two runs from claiming the same post.
func (s *Posts) ClaimDue(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]social.Post, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := s.DB.Query(ctx, `
		UPDATE social_posts SET status = 'publishing', next_attempt_at = $3
		WHERE id IN (
			SELECT id FROM social_posts
			WHERE draft = FALSE
				AND publish_at <= $1
				AND status IN ('scheduled', 'partial', 'publishing')
				AND next_attempt_at <= $1
			ORDER BY publish_at
			LIMIT $2
			FOR UPDATE SKIP LOCKED)
		RETURNING `+postColumns,
		now, limit, now.Add(lease))
	if err != nil {
		return nil, fmt.Errorf("social: claim due posts: %w", err)
	}
	defer rows.Close()

	var out []social.Post
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("social: claim due posts: %w", err)
	}
	return out, nil
}

// RecordResult writes a post's publishing state back. The content is not
// touched: a run never changes what it publishes.
func (s *Posts) RecordResult(ctx context.Context, p social.Post) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	_, err := s.DB.Exec(ctx, `
		UPDATE social_posts SET
			status          = $2,
			ig_media_id     = $3,
			fb_post_id      = $4,
			attempts        = $5,
			last_error      = $6,
			next_attempt_at = $7,
			published_at    = $8,
			threads_media_id = $9
		WHERE id = $1`,
		p.ID, string(p.Status), nullable(p.IGMediaID), nullable(p.FBPostID), p.Attempts,
		nullable(p.LastError), p.NextAttemptAt, p.PublishedAt, nullable(p.ThreadsMediaID))
	if err != nil {
		return fmt.Errorf("social: record publish result: %w", err)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanPost(row scanner) (social.Post, error) {
	var (
		p          social.Post
		networks   []string
		kind       string
		status     string
		mediaJSON  []byte
		igID       *string
		fbID       *string
		lastErr    *string
		nextAt     *time.Time
		publishedA *time.Time
		threadsID  *string
		threadsTxt *string
		topic      *string
		coverJSON  []byte
	)
	err := row.Scan(&p.ID, &p.PublishAt, &networks, &kind, &p.Caption, &p.AltText, &mediaJSON, &p.Draft, &status,
		&igID, &fbID, &p.Attempts, &lastErr, &nextAt, &publishedA, &threadsID, &threadsTxt, &topic, &coverJSON)
	if err != nil {
		return social.Post{}, fmt.Errorf("social: scan post: %w", err)
	}
	if err := json.Unmarshal(mediaJSON, &p.Media); err != nil {
		return social.Post{}, fmt.Errorf("social: decode media of %s: %w", p.ID, err)
	}
	if len(coverJSON) > 0 {
		var cover social.Media
		if err := json.Unmarshal(coverJSON, &cover); err != nil {
			return social.Post{}, fmt.Errorf("social: decode cover of %s: %w", p.ID, err)
		}
		p.Cover = &cover
	}
	p.Networks = toNetworks(networks)
	p.Kind = social.Kind(kind)
	p.Status = social.Status(status)
	p.IGMediaID = deref(igID)
	p.FBPostID = deref(fbID)
	p.LastError = deref(lastErr)
	p.NextAttemptAt = nextAt
	p.PublishedAt = publishedA
	p.ThreadsMediaID = deref(threadsID)
	p.ThreadsText = deref(threadsTxt)
	p.ThreadsTopic = deref(topic)
	return p, nil
}

func networkStrings(networks []social.Network) []string {
	out := make([]string, 0, len(networks))
	for _, n := range networks {
		out = append(out, string(n))
	}
	return out
}

func toNetworks(raw []string) []social.Network {
	out := make([]social.Network, 0, len(raw))
	for _, r := range raw {
		out = append(out, social.Network(r))
	}
	return out
}

func nonNilMedia(m []social.Media) []social.Media {
	if m == nil {
		return []social.Media{}
	}
	return m
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
