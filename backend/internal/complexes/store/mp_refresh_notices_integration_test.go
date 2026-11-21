//go:build integration

package store_test

import (
	"context"
	"testing"

	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// The refresh-failure marker answers "told" only for the episode it recorded,
// a new episode replaces it, and it is scoped to its own complex.
func TestIntegration_MPRefreshFailureNoticeIsRememberedPerEpisode(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	complexes := f.Stores.Complexes

	told, err := complexes.MPRefreshFailureNotified(ctx, f.ComplexID, "episode-1")
	if err != nil {
		t.Fatalf("MPRefreshFailureNotified on a fresh complex: %v", err)
	}
	if told {
		t.Fatal("a complex nobody was told about reads as told")
	}

	if err := complexes.RecordMPRefreshFailureNotice(ctx, f.ComplexID, "episode-1"); err != nil {
		t.Fatalf("RecordMPRefreshFailureNotice: %v", err)
	}
	if told, err := complexes.MPRefreshFailureNotified(ctx, f.ComplexID, "episode-1"); err != nil || !told {
		t.Fatalf("the recorded episode reads as not told (told=%v, err=%v)", told, err)
	}
	if told, _ := complexes.MPRefreshFailureNotified(ctx, f.ComplexID, "episode-2"); told {
		t.Error("a new episode reads as already told")
	}

	if err := complexes.RecordMPRefreshFailureNotice(ctx, f.ComplexID, "episode-2"); err != nil {
		t.Fatalf("recording the second episode: %v", err)
	}
	if told, _ := complexes.MPRefreshFailureNotified(ctx, f.ComplexID, "episode-2"); !told {
		t.Error("the second episode was not recorded over the first")
	}
}
