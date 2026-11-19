//go:build integration

package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	"github.com/stodulski/vibe-server/internal/stores"
)

// storyDay is the venue day the tests' daily stories belong to.
var storyDay = time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)

func freedStory(complexID uuid.UUID, key string, day, now time.Time) complexstore.NewStory {
	starts := now.Add(3 * time.Hour)
	return complexstore.NewStory{
		ComplexID: complexID, Kind: complexstore.StoryKindFreed, DedupKey: key,
		ImageURL: "https://vibe.test/historias/c/x/liberado.jpg", QueuedDay: day,
		SlotStartsAt: &starts, ScheduledFor: now,
	}
}

// Settings read as the defaults until saved, then as saved; the table refuses
// a time or a cap outside the allowed range even if a caller skipped the
// handler's validation.
func TestIntegration_InstagramStorySettingsDefaultAndRoundTrip(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	s := f.Stores.InstagramStories

	got, err := s.GetStorySettings(ctx, f.ComplexID)
	if err != nil || got != complexstore.DefaultStorySettings() {
		t.Fatalf("unsaved settings = %+v, %v; want the defaults", got, err)
	}
	want := complexstore.StorySettings{DailyEnabled: true, DailyTime: "21:30", FreedEnabled: true, FreedDailyCap: 7}
	if _, err := s.SaveStorySettings(ctx, f.ComplexID, want, storyDay); err != nil {
		t.Fatalf("SaveStorySettings: %v", err)
	}
	if got, _ := s.GetStorySettings(ctx, f.ComplexID); got != want {
		t.Fatalf("saved settings read back as %+v, want %+v", got, want)
	}
	if _, err := s.SaveStorySettings(ctx, f.ComplexID, complexstore.StorySettings{DailyTime: "23:00", FreedDailyCap: 5}, storyDay); err == nil {
		t.Fatal("a daily time of 23:00 was stored")
	}
}

// A freed story is queued only with freed stories on, at most cap per venue
// day, and once per dedup key ever.
func TestIntegration_FreedStoriesRespectTheSwitchTheCapAndTheDedupKey(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	s := f.Stores.InstagramStories
	now := time.Now()
	day := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)

	if ok, err := s.QueueFreedStory(ctx, freedStory(f.ComplexID, "freed:a", day, now)); err != nil || ok {
		t.Fatalf("with no settings saved: queued=%v err=%v, want nothing queued", ok, err)
	}
	if _, err := s.SaveStorySettings(ctx, f.ComplexID, complexstore.StorySettings{DailyTime: "10:00", FreedEnabled: false, FreedDailyCap: 2}, storyDay); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.QueueFreedStory(ctx, freedStory(f.ComplexID, "freed:a", day, now)); ok {
		t.Fatal("queued with freed stories off")
	}
	if _, err := s.SaveStorySettings(ctx, f.ComplexID, complexstore.StorySettings{DailyTime: "10:00", FreedEnabled: true, FreedDailyCap: 2}, storyDay); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		key  string
		day  time.Time
		want bool
	}{
		{"freed:a", day, true},
		{"freed:a", day, false}, // same slot again
		{"freed:b", day, true},
		{"freed:c", day, false},                 // cap of 2 reached
		{"freed:c", day.AddDate(0, 0, 1), true}, // a new venue day
		{"freed:a", day.AddDate(0, 0, 1), false},
	} {
		ok, err := s.QueueFreedStory(ctx, freedStory(f.ComplexID, tc.key, tc.day, now))
		if err != nil || ok != tc.want {
			t.Fatalf("%s on %s: queued=%v err=%v, want %v", tc.key, tc.day.Format("2006-01-02"), ok, err, tc.want)
		}
	}
}

// A complex's daily story is due once its time has come, while its connection
// is usable, and until it is queued for that day.
func TestIntegration_DailyStoryIsDueOncePerDayAfterItsTime(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := data.ContextWithTenantBypass(context.Background())
	s := f.Stores.InstagramStories
	now := time.Now()
	saveIG(t, ctx, f.Stores.Instagram, f.ComplexID, "IGAAdaily-token", now.Add(30*24*time.Hour))
	if _, err := s.SaveStorySettings(ctx, f.ComplexID, complexstore.StorySettings{DailyEnabled: true, DailyTime: "10:00", FreedDailyCap: 5}, storyDay); err != nil {
		t.Fatal(err)
	}
	due := func(hhmm string) bool {
		t.Helper()
		list, err := s.ListDailyStoriesDue(ctx, now, storyDay, hhmm, "daily:2026-11-20", 10_000)
		if err != nil {
			t.Fatalf("ListDailyStoriesDue: %v", err)
		}
		for _, d := range list {
			if d.ComplexID == f.ComplexID {
				return d.Slug != ""
			}
		}
		return false
	}
	if due("09:59") {
		t.Fatal("due before its time")
	}
	if !due("10:00") || !due("15:00") {
		t.Fatal("not due at or after its time")
	}
	if _, err := f.DB.Exec(ctx, `UPDATE instagram_connections SET needs_reconnect = true WHERE complex_id = $1`, f.ComplexID); err != nil {
		t.Fatal(err)
	}
	if due("15:00") {
		t.Fatal("due with a connection that needs reconnecting")
	}
	if _, err := f.DB.Exec(ctx, `UPDATE instagram_connections SET needs_reconnect = false WHERE complex_id = $1`, f.ComplexID); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.QueueStory(ctx, complexstore.NewStory{
		ComplexID: f.ComplexID, Kind: complexstore.StoryKindDaily, DedupKey: "daily:2026-11-20",
		ImageURL: "https://vibe.test/x.jpg", QueuedDay: now, ScheduledFor: now,
	}); err != nil || !ok {
		t.Fatalf("QueueStory: %v %v", ok, err)
	}
	if due("15:00") {
		t.Fatal("still due after today's story was queued")
	}
	list, _ := s.ListDailyStoriesDue(ctx, now, storyDay.AddDate(0, 0, 1), "15:00", "daily:2026-11-21", 10_000)
	found := false
	for _, d := range list {
		found = found || d.ComplexID == f.ComplexID
	}
	if !found {
		t.Fatal("not due on the next day")
	}
}

// A claim leases the due pending stories: a second claim inside the lease gets
// none of them, a story not due yet is not claimed, and saving a state releases
// the lease.
func TestIntegration_ClaimLeasesDueStories(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := data.ContextWithTenantBypass(context.Background())
	s := f.Stores.InstagramStories
	now := time.Now()
	if _, err := s.SaveStorySettings(ctx, f.ComplexID, complexstore.StorySettings{DailyTime: "10:00", FreedEnabled: true, FreedDailyCap: 5}, storyDay); err != nil {
		t.Fatal(err)
	}
	if _, err := s.QueueFreedStory(ctx, freedStory(f.ComplexID, "freed:due", now, now.Add(-time.Minute))); err != nil {
		t.Fatal(err)
	}
	if _, err := s.QueueFreedStory(ctx, freedStory(f.ComplexID, "freed:later", now, now.Add(time.Hour))); err != nil {
		t.Fatal(err)
	}
	mine := func(list []complexstore.QueuedStory) []complexstore.QueuedStory {
		var out []complexstore.QueuedStory
		for _, q := range list {
			if q.ComplexID == f.ComplexID {
				out = append(out, q)
			}
		}
		return out
	}
	first, err := s.ClaimDueStories(ctx, now, now.Add(3*time.Minute), 100)
	if err != nil {
		t.Fatal(err)
	}
	claimed := mine(first)
	if len(claimed) != 1 || claimed[0].Kind != complexstore.StoryKindFreed || claimed[0].SlotStartsAt == nil {
		t.Fatalf("first claim = %+v, want the one due story", claimed)
	}
	// Recording the container keeps the lease.
	if err := s.SaveStoryContainer(ctx, claimed[0].ID, "c-0", now); err != nil {
		t.Fatalf("SaveStoryContainer: %v", err)
	}
	if again, _ := s.ClaimDueStories(ctx, now.Add(time.Minute), now.Add(4*time.Minute), 100); len(mine(again)) != 0 {
		t.Fatalf("a leased story was claimed again: %+v", mine(again))
	}
	if err := s.SaveStoryState(ctx, claimed[0].ID, complexstore.StoryState{
		Status: complexstore.StoryPending, Attempts: 1, NextAttemptAt: now, ContainerID: "c-1", ContainerCreatedAt: &now,
	}); err != nil {
		t.Fatal(err)
	}
	after, _ := s.ClaimDueStories(ctx, now.Add(time.Minute), now.Add(4*time.Minute), 100)
	if got := mine(after); len(got) != 1 || got[0].ContainerID != "c-1" || got[0].Attempts != 1 {
		t.Fatalf("after saving: %+v, want the story back with its container and attempt", got)
	}

	recent, err := s.ListRecentStories(f.Scoped(context.Background()), f.ComplexID, 10)
	if err != nil || len(recent) != 2 || recent[0].ScheduledFor.Before(recent[1].ScheduledFor) {
		t.Fatalf("recent = %+v, %v; want both stories, newest first", recent, err)
	}
}

// dueToday reports whether the complex's daily story is due at hhmm on day.
func dueToday(t *testing.T, ctx context.Context, s stores.InstagramStoryStore, complexID uuid.UUID, day time.Time, hhmm string) bool {
	t.Helper()
	list, err := s.ListDailyStoriesDue(ctx, time.Now(), day, hhmm, "daily:"+day.Format("2006-01-02"), 10_000)
	if err != nil {
		t.Fatalf("ListDailyStoriesDue: %v", err)
	}
	for _, d := range list {
		if d.ComplexID == complexID {
			return true
		}
	}
	return false
}

// S8.5: a daily story turned on after its time starts on the day the save
// that turned it on named; a later save while it stays on (a later
// daily_time) keeps that day, and only turning it off and on again moves it.
func TestIntegration_DailyStoryTurnedOnAfterItsTimeStartsTheNextDay(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := data.ContextWithTenantBypass(context.Background())
	s := f.Stores.InstagramStories
	saveIG(t, ctx, f.Stores.Instagram, f.ComplexID, "IGAAstart-token", time.Now().Add(30*24*time.Hour))
	today, tomorrow := storyDay, storyDay.AddDate(0, 0, 1)
	save := func(enabled bool, dailyTime string, startsOn time.Time) {
		t.Helper()
		if _, err := s.SaveStorySettings(ctx, f.ComplexID, complexstore.StorySettings{
			DailyEnabled: enabled, DailyTime: dailyTime, FreedDailyCap: 5,
		}, startsOn); err != nil {
			t.Fatalf("SaveStorySettings: %v", err)
		}
	}

	// Turned on at 15:00 with 10:00 as its time: the handler says tomorrow.
	save(true, "10:00", tomorrow)
	if dueToday(t, ctx, s, f.ComplexID, today, "15:00") {
		t.Fatal("turned on after its time, it is due today")
	}
	// Moved to 18:00 later the same day: the handler now says today, the
	// stored start stays tomorrow.
	save(true, "18:00", today)
	if dueToday(t, ctx, s, f.ComplexID, today, "18:30") {
		t.Fatal("moving the time later made today's story due")
	}
	if !dueToday(t, ctx, s, f.ComplexID, tomorrow, "18:30") {
		t.Fatal("not due on the day it starts")
	}
	// Off, then on again before its time: it starts today.
	save(false, "18:00", today)
	save(true, "18:00", today)
	if !dueToday(t, ctx, s, f.ComplexID, today, "18:30") {
		t.Fatal("turned on again before its time, it is not due today")
	}
}

// S8.6: disconnecting Instagram turns both story switches off in the same
// statement that deletes the connection.
func TestIntegration_DisconnectTurnsTheStoriesOff(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	saveIG(t, ctx, f.Stores.Instagram, f.ComplexID, "IGAAoff-token", time.Now().Add(30*24*time.Hour))
	if _, err := f.Stores.InstagramStories.SaveStorySettings(ctx, f.ComplexID, complexstore.StorySettings{
		DailyEnabled: true, DailyTime: "12:00", FreedEnabled: true, FreedDailyCap: 7,
	}, storyDay); err != nil {
		t.Fatal(err)
	}

	if deleted, err := f.Stores.Instagram.DeleteInstagramConnection(ctx, f.ComplexID); err != nil || !deleted {
		t.Fatalf("DeleteInstagramConnection = (%v, %v), want (true, nil)", deleted, err)
	}

	got, err := f.Stores.InstagramStories.GetStorySettings(ctx, f.ComplexID)
	if err != nil {
		t.Fatal(err)
	}
	want := complexstore.StorySettings{DailyEnabled: false, DailyTime: "12:00", FreedEnabled: false, FreedDailyCap: 7}
	if got != want {
		t.Fatalf("settings after disconnect = %+v, want %+v", got, want)
	}
}

// S8.9: the publisher's reconnect flag applies only to the token that failed:
// a connection renewed since (another expiry) is left alone.
func TestIntegration_StoryReconnectFlagMatchesTheFailedToken(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	old := time.Now().Add(10 * 24 * time.Hour).Truncate(time.Second)
	renewed := old.Add(50 * 24 * time.Hour)
	saveIG(t, ctx, f.Stores.Instagram, f.ComplexID, "IGAArenewed-token", renewed)
	s := f.Stores.InstagramStories

	if err := s.MarkStoryAccountNeedsReconnect(ctx, f.ComplexID, old); err != nil {
		t.Fatal(err)
	}
	if acct, _ := s.StoryAccount(ctx, f.ComplexID); acct.NeedsReconnect {
		t.Fatal("the old token's failure flagged the renewed connection")
	}
	if err := s.MarkStoryAccountNeedsReconnect(ctx, f.ComplexID, renewed); err != nil {
		t.Fatal(err)
	}
	if acct, _ := s.StoryAccount(ctx, f.ComplexID); !acct.NeedsReconnect {
		t.Fatal("the current token's failure did not flag the connection")
	}
}
