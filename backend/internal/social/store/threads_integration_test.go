//go:build integration

package store_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/crypto"
	"github.com/stodulski/vibe-server/internal/data/datatest"
	"github.com/stodulski/vibe-server/internal/social"
	"github.com/stodulski/vibe-server/internal/social/store"
)

func integrationKeys(t *testing.T) *crypto.Keyring {
	t.Helper()
	keys, err := crypto.ParseKeyring("k1:" + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32)))
	if err != nil {
		t.Fatalf("keyring: %v", err)
	}
	return keys
}

func TestIntegration_ThreadsTokenIsSealedAtRestAndReadBack(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	threads := store.NewThreads(f.DB, integrationKeys(t))

	if _, ok, err := threads.LoadThreads(ctx); err != nil || ok {
		t.Fatalf("an empty table must report no connection; ok=%v err=%v", ok, err)
	}
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	conn := social.ThreadsConnection{
		UserID: "42", Username: "vibe.padel", AccessToken: "long-token-value",
		ConnectedAt: now, RefreshedAt: now, ExpiresAt: now.Add(60 * 24 * time.Hour),
	}
	if err := threads.SaveThreads(ctx, conn); err != nil {
		t.Fatalf("SaveThreads() = %v", err)
	}

	var raw string
	if err := f.DB.QueryRow(ctx, `SELECT access_token FROM social_threads_connection WHERE id = 1`).Scan(&raw); err != nil {
		t.Fatalf("read raw column: %v", err)
	}
	if strings.Contains(raw, "long-token-value") || !strings.HasPrefix(raw, "v1.") {
		t.Fatalf("the column holds %q; want a sealed envelope, never the token", raw)
	}

	got, ok, err := threads.LoadThreads(ctx)
	if err != nil || !ok {
		t.Fatalf("LoadThreads() ok=%v err=%v", ok, err)
	}
	if got.AccessToken != conn.AccessToken || got.UserID != "42" || got.Username != "vibe.padel" || !got.ExpiresAt.Equal(conn.ExpiresAt) {
		t.Fatalf("loaded %+v; want the connection saved", got)
	}
}

func TestIntegration_ThreadsSaveReplacesTheSingleConnection(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	threads := store.NewThreads(f.DB, integrationKeys(t))
	first := social.ThreadsConnection{UserID: "1", Username: "old", AccessToken: "old-token", ConnectedAt: due, RefreshedAt: due, ExpiresAt: due}
	second := social.ThreadsConnection{UserID: "2", Username: "new", AccessToken: "new-token", ConnectedAt: due, RefreshedAt: due, ExpiresAt: due}

	if err := threads.SaveThreads(ctx, first); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if err := threads.SaveThreads(ctx, second); err != nil {
		t.Fatalf("second save: %v", err)
	}
	got, ok, err := threads.LoadThreads(ctx)
	if err != nil || !ok || got.Username != "new" || got.AccessToken != "new-token" {
		t.Fatalf("loaded %+v ok=%v err=%v; want the second connection only", got, ok, err)
	}
	var rows int
	if err := f.DB.QueryRow(ctx, `SELECT count(*) FROM social_threads_connection`).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("rows = %d, err = %v; want exactly one", rows, err)
	}
}

func TestIntegration_ThreadsFieldsAndCoverRoundTripAndFreeze(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	posts := store.NewPosts(f.DB)

	p := samplePost("threads-1", social.NetworkInstagram, social.NetworkThreads)
	p.Kind = social.TypeReel
	p.Media = []social.Media{{Name: "clip.mp4", URL: "https://cdn.test/social/v.mp4", ContentType: "video/mp4", SHA256: "v", SizeBytes: 9}}
	p.Cover = &social.Media{Name: "cover.jpg", URL: "https://cdn.test/social/c.jpg", ContentType: "image/jpeg", SHA256: "c", SizeBytes: 5}
	p.ThreadsText = "Texto para Threads"
	p.ThreadsTopic = "padel"
	if err := posts.Upsert(ctx, p); err != nil {
		t.Fatalf("Upsert() = %v", err)
	}

	claimed, err := posts.ClaimDue(ctx, due.Add(time.Minute), 10, 15*time.Minute)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("ClaimDue() = %d posts, %v", len(claimed), err)
	}
	got := claimed[0]
	if got.Cover == nil || got.Cover.Name != "cover.jpg" || got.ThreadsText != "Texto para Threads" || got.ThreadsTopic != "padel" {
		t.Fatalf("claimed = %+v; want the cover and the Threads fields back", got)
	}

	got.ThreadsMediaID = "th-1"
	got.Status = social.StatusPartial
	if err := posts.RecordResult(ctx, got); err != nil {
		t.Fatalf("RecordResult() = %v", err)
	}
	frozen, err := posts.IsFrozen(ctx, "threads-1")
	if err != nil || !frozen {
		t.Fatalf("IsFrozen() = %v, %v; a Threads publish must freeze the post", frozen, err)
	}
	if err := posts.Upsert(ctx, p); !errors.Is(err, social.ErrFrozen) {
		t.Fatalf("Upsert() after a Threads publish = %v; want ErrFrozen", err)
	}
}

func TestIntegration_TextPostsAreThreadsOnlyInTheDatabase(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	posts := store.NewPosts(f.DB)

	text := samplePost("solo", social.NetworkInstagram)
	text.Kind = social.TypeText
	text.Media = nil
	err := posts.Upsert(ctx, text)
	if err == nil || errors.Is(err, social.ErrFrozen) {
		t.Fatalf("Upsert() = %v; a text post naming instagram must be refused by the database", err)
	}

	text.Networks = []social.Network{social.NetworkThreads}
	if err := posts.Upsert(ctx, text); err != nil {
		t.Fatalf("a threads-only text post must be stored: %v", err)
	}
}

func TestIntegration_ThreadsDisconnectRemovesOnlyTheMatchingAccount(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	threads := store.NewThreads(f.DB, integrationKeys(t))
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	conn := social.ThreadsConnection{
		UserID: "42", Username: "vibe.padel", AccessToken: "long-token-value",
		ConnectedAt: now, RefreshedAt: now, ExpiresAt: now.Add(60 * 24 * time.Hour),
		KeywordSearchRequested: true,
	}
	if err := threads.SaveThreads(ctx, conn); err != nil {
		t.Fatalf("SaveThreads() = %v", err)
	}

	username, removed, err := threads.DisconnectThreads(ctx, "99")
	if err != nil || removed || username != "" {
		t.Fatalf("another account's disconnect = (%q, %v, %v); want nothing removed", username, removed, err)
	}
	got, ok, err := threads.LoadThreads(ctx)
	if err != nil || !ok || !got.KeywordSearchRequested {
		t.Fatalf("connection after a foreign disconnect = %+v, ok %v, err %v; want it kept with its keyword flag", got, ok, err)
	}

	username, removed, err = threads.DisconnectThreads(ctx, "42")
	if err != nil || !removed || username != "vibe.padel" {
		t.Fatalf("disconnect = (%q, %v, %v); want the account removed", username, removed, err)
	}
	if _, ok, _ := threads.LoadThreads(ctx); ok {
		t.Fatal("the connection must be gone after disconnect")
	}
	if _, removed, _ := threads.DisconnectThreads(ctx, "42"); removed {
		t.Fatal("a repeated disconnect must find nothing to remove")
	}
}

func TestIntegration_ThreadsKeywordSearchFlagRoundTrips(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	threads := store.NewThreads(f.DB, integrationKeys(t))
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	conn := social.ThreadsConnection{
		UserID: "42", Username: "vibe.padel", AccessToken: "long-token-value",
		ConnectedAt: now, RefreshedAt: now, ExpiresAt: now.Add(60 * 24 * time.Hour),
	}
	if err := threads.SaveThreads(ctx, conn); err != nil {
		t.Fatalf("SaveThreads() = %v", err)
	}
	got, _, _ := threads.LoadThreads(ctx)
	if got.KeywordSearchRequested {
		t.Fatal("a connection saved without the keyword scope must read back as not requested")
	}
	conn.KeywordSearchRequested = true
	if err := threads.SaveThreads(ctx, conn); err != nil {
		t.Fatalf("SaveThreads() = %v", err)
	}
	got, _, _ = threads.LoadThreads(ctx)
	if !got.KeywordSearchRequested {
		t.Fatal("the keyword flag must survive a save and load")
	}
}
