package social

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestKeywordSearchCheckDescribesEachApprovalState(t *testing.T) {
	t.Parallel()
	const kw = "complejo de pádel"
	foreign := keywordPost("f1", "jugador.a", 0, false, "hola")
	own := keywordPost("o1", "vibe.padel", 0, false, "nuestro post")

	tests := []struct {
		name    string
		store   *fakeThreadsStore
		search  *fakeSearch
		want    string
		problem bool
		calls   int
	}{
		{
			name:   "approved: another account's post is found",
			store:  digestConn(true),
			search: &fakeSearch{results: map[string][]ThreadsKeywordPost{kw + "|TOP": {own, foreign}}},
			want:   "Búsqueda por palabra clave: aprobada.",
			calls:  1,
		},
		{
			name:   "pending: Meta only returns our own posts",
			store:  digestConn(true),
			search: &fakeSearch{results: map[string][]ThreadsKeywordPost{kw + "|TOP": {own}}},
			want:   "pendiente de aprobación de Meta",
			calls:  1,
		},
		{
			name:    "no permission: the consent did not ask for keyword search",
			store:   digestConn(false),
			search:  &fakeSearch{},
			want:    "sin permiso",
			problem: true,
		},
		{
			name:   "not connected",
			store:  &fakeThreadsStore{},
			search: &fakeSearch{},
			want:   "sin permiso",
		},
		{
			name:    "the probe fails",
			store:   digestConn(true),
			search:  &fakeSearch{err: errors.New("threads answered 500")},
			want:    "no se pudo verificar",
			problem: true,
			calls:   1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			check := NewThreadsSearchCheck(tc.store, tc.search, []string{kw})
			line := check.Check(context.Background(), ConnectionReport{})
			if !strings.Contains(line.Text, tc.want) || line.Problem != tc.problem {
				t.Fatalf("line = %+v; want text containing %q, problem %v", line, tc.want, tc.problem)
			}
			if len(tc.search.calls) != tc.calls {
				t.Fatalf("searches = %d; want %d", len(tc.search.calls), tc.calls)
			}
		})
	}
}

func TestKeywordSearchCheckNeverShowsTheToken(t *testing.T) {
	t.Parallel()
	search := &fakeSearch{err: errors.New("boom")}
	line := NewThreadsSearchCheck(digestConn(true), search, []string{"complejo de pádel"}).Check(context.Background(), ConnectionReport{})
	if strings.Contains(line.Text, "secret-long-token") {
		t.Fatal("the status line must never carry the token")
	}
}
