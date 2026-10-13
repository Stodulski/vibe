package social

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/timezone"
)

// digestNow is a Monday at 09:30 in Buenos Aires, inside the weekly window.
var digestNow = time.Date(2026, time.October, 12, 9, 30, 0, 0, timezone.Argentina)

type fakeSearch struct {
	results map[string][]ThreadsKeywordPost
	err     error
	calls   []string
}

func (f *fakeSearch) KeywordSearch(_ context.Context, query, searchType string) ([]ThreadsKeywordPost, error) {
	f.calls = append(f.calls, query+"|"+searchType)
	if f.err != nil {
		return nil, f.err
	}
	return f.results[query+"|"+searchType], nil
}

func digestConn(requested bool) *fakeThreadsStore {
	return &fakeThreadsStore{ok: true, conn: ThreadsConnection{
		UserID: "42", Username: "vibe.padel", AccessToken: "secret-long-token", KeywordSearchRequested: requested,
	}}
}

func newDigestAt(now time.Time, store *fakeThreadsStore, search *fakeSearch, q *fakeEnqueuer, keywords ...string) *ThreadsDigest {
	logger := slog.New(slog.DiscardHandler)
	return NewThreadsDigest(store, search, q, ThreadsDigestConfig{
		To:       "ops@vibe.test",
		Keywords: keywords,
		Now:      func() time.Time { return now },
	}, logger)
}

func keywordPost(id, user string, ago time.Duration, replies bool, text string) ThreadsKeywordPost {
	return ThreadsKeywordPost{
		ID: id, Username: user, Text: text, HasReplies: replies,
		Permalink: "https://www.threads.net/@" + user + "/post/" + id,
		Timestamp: digestNow.Add(-ago),
	}
}

func TestDigestRunsOnlyInTheMondayMorningWindow(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		now  time.Time
		due  bool
	}{
		{name: "monday 08:59", now: time.Date(2026, 10, 12, 8, 59, 0, 0, timezone.Argentina), due: false},
		{name: "monday 09:00", now: time.Date(2026, 10, 12, 9, 0, 0, 0, timezone.Argentina), due: true},
		{name: "monday 10:59", now: time.Date(2026, 10, 12, 10, 59, 0, 0, timezone.Argentina), due: true},
		{name: "monday 11:00", now: time.Date(2026, 10, 12, 11, 0, 0, 0, timezone.Argentina), due: false},
		{name: "tuesday 09:30", now: time.Date(2026, 10, 13, 9, 30, 0, 0, timezone.Argentina), due: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			search := &fakeSearch{}
			q := &fakeEnqueuer{}
			outcome, err := newDigestAt(tc.now, digestConn(true), search, q, "complejo de pádel").Run(context.Background())
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if tc.due == (outcome == ThreadsDigestNotDue) {
				t.Fatalf("outcome = %q with due = %v", outcome, tc.due)
			}
			if !tc.due && (len(search.calls) != 0 || len(q.calls) != 0) {
				t.Fatal("outside the window the digest must not search or mail")
			}
		})
	}
}

func TestDigestDoesNothingWithoutAConnection(t *testing.T) {
	t.Parallel()
	search := &fakeSearch{}
	q := &fakeEnqueuer{}
	outcome, err := newDigestAt(digestNow, &fakeThreadsStore{}, search, q, "complejo de pádel").Run(context.Background())
	if err != nil || outcome != ThreadsDigestNoConnection {
		t.Fatalf("outcome = %q, err = %v; want no connection", outcome, err)
	}
	if len(search.calls) != 0 || len(q.calls) != 0 {
		t.Fatal("without a connection there is nothing to search with or mail about")
	}
}

func TestDigestAsksNothingWhenTheConsentDidNotRequestKeywordSearch(t *testing.T) {
	t.Parallel()
	search := &fakeSearch{}
	q := &fakeEnqueuer{}
	outcome, err := newDigestAt(digestNow, digestConn(false), search, q, "complejo de pádel").Run(context.Background())
	if err != nil || outcome != ThreadsDigestNotRequested {
		t.Fatalf("outcome = %q, err = %v; want not requested", outcome, err)
	}
	if len(search.calls) != 0 || len(q.calls) != 0 {
		t.Fatal("a connection made before the keyword scope existed must not search or mail")
	}
}

func TestDigestSendsNothingWhileKeywordSearchIsNotApproved(t *testing.T) {
	t.Parallel()
	own := keywordPost("o1", "vibe.padel", time.Hour, false, "nuestra publicación")
	search := &fakeSearch{results: map[string][]ThreadsKeywordPost{
		"complejo de pádel|TOP":    {own},
		"complejo de pádel|RECENT": {own},
	}}
	q := &fakeEnqueuer{}
	outcome, err := newDigestAt(digestNow, digestConn(true), search, q, "complejo de pádel").Run(context.Background())
	if err != nil || outcome != ThreadsDigestEmpty {
		t.Fatalf("outcome = %q, err = %v; want empty", outcome, err)
	}
	if len(q.calls) != 0 {
		t.Fatalf("emails = %d; want none while Meta only returns our own posts", len(q.calls))
	}
}

func TestDigestGroupsApprovedResultsByKeywordAndDropsOwnAndOldPosts(t *testing.T) {
	t.Parallel()
	const kwPadel, kwCancha = "complejo de pádel", "reservar cancha"
	conversation := keywordPost("a", "jugador.a", 2*24*time.Hour, true, "¿Alguien sabe cuánto sale el turno en el complejo?")
	quiet := keywordPost("b", "jugador.b", 3*24*time.Hour, false, "Buscamos cancha para el sábado")
	old := keywordPost("old", "jugador.c", 8*24*time.Hour, true, "post de hace más de una semana")
	own := keywordPost("own", "vibe.padel", time.Hour, true, "nuestro propio post")
	booking := keywordPost("c", "jugador.d", 24*time.Hour, false, "Cómo reservar cancha en la app")

	search := &fakeSearch{results: map[string][]ThreadsKeywordPost{
		kwPadel + "|TOP":    {conversation, own, old},
		kwPadel + "|RECENT": {conversation, quiet},
		kwCancha + "|TOP":   {booking},
	}}
	q := &fakeEnqueuer{}
	outcome, err := newDigestAt(digestNow, digestConn(true), search, q, kwPadel, kwCancha).Run(context.Background())
	if err != nil || outcome != ThreadsDigestQueued {
		t.Fatalf("outcome = %q, err = %v; want queued", outcome, err)
	}
	if len(q.calls) != 1 {
		t.Fatalf("emails = %d; want one grouped email", len(q.calls))
	}
	mail := q.calls[0].payload
	if mail.Subject != "Threads: conversaciones de la semana" {
		t.Fatalf("subject = %q", mail.Subject)
	}
	body := strings.Join(mail.Lines, "\n")
	for _, want := range []string{
		"Palabra clave: " + kwPadel,
		"Palabra clave: " + kwCancha,
		"@jugador.a", "@jugador.b", "@jugador.d",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("email body lacks %q:\n%s", want, body)
		}
	}
	for _, unwanted := range []string{"@vibe.padel", "@jugador.c", "secret-long-token"} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("email body must not contain %q:\n%s", unwanted, body)
		}
	}
	if n := strings.Count(body, conversation.Permalink); n != 1 {
		t.Fatalf("conversation listed %d times; want once after dedupe across modes", n)
	}
	if strings.Index(body, "Palabra clave: "+kwPadel) > strings.Index(body, "Palabra clave: "+kwCancha) {
		t.Fatal("keyword groups must follow the configured keyword order")
	}
	if strings.Index(body, conversation.Permalink) > strings.Index(body, quiet.Permalink) {
		t.Fatal("a post with replies must rank before one without")
	}
}

func TestDigestCapsTheEmailAtThirtyPosts(t *testing.T) {
	t.Parallel()
	var posts []ThreadsKeywordPost
	for i := range 35 {
		posts = append(posts, keywordPost(fmt.Sprintf("p%02d", i), fmt.Sprintf("u%02d", i), time.Duration(i+1)*time.Hour, false, "texto"))
	}
	search := &fakeSearch{results: map[string][]ThreadsKeywordPost{"complejo de pádel|TOP": posts}}
	q := &fakeEnqueuer{}
	if _, err := newDigestAt(digestNow, digestConn(true), search, q, "complejo de pádel").Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(q.calls) != 1 {
		t.Fatalf("emails = %d; want 1", len(q.calls))
	}
	if n := strings.Count(strings.Join(q.calls[0].payload.Lines, "\n"), "https://www.threads.net/"); n != 30 {
		t.Fatalf("listed %d posts; want the cap of 30", n)
	}
}

func TestDigestExcerptsEachTextToTwoHundredCharacters(t *testing.T) {
	t.Parallel()
	long := keywordPost("long", "jugador.e", time.Hour, false, strings.Repeat("x", 220))
	search := &fakeSearch{results: map[string][]ThreadsKeywordPost{"complejo de pádel|TOP": {long}}}
	q := &fakeEnqueuer{}
	if _, err := newDigestAt(digestNow, digestConn(true), search, q, "complejo de pádel").Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	body := strings.Join(q.calls[0].payload.Lines, "\n")
	if !strings.Contains(body, strings.Repeat("x", 200)+"…") || strings.Contains(body, strings.Repeat("x", 201)) {
		t.Fatalf("excerpt must be the first 200 characters and an ellipsis:\n%s", body)
	}
}

func TestDigestDedupKeyIsOnePerISOWeek(t *testing.T) {
	t.Parallel()
	q := &fakeEnqueuer{}
	run := func(now time.Time) {
		t.Helper()
		// A post from an hour before this run, so each week's window holds it.
		post := ThreadsKeywordPost{ID: "a", Username: "jugador.a", Text: "hola", Permalink: "https://www.threads.net/@jugador.a/post/a", Timestamp: now.Add(-time.Hour)}
		search := &fakeSearch{results: map[string][]ThreadsKeywordPost{"complejo de pádel|TOP": {post}}}
		if _, err := newDigestAt(now, digestConn(true), search, q, "complejo de pádel").Run(context.Background()); err != nil {
			t.Fatalf("Run(%v) error = %v", now, err)
		}
	}
	run(digestNow)
	run(digestNow.Add(time.Hour)) // a second run in the same week
	nextWeek := digestNow.AddDate(0, 0, 7)
	run(nextWeek)

	if len(q.calls) != 3 {
		t.Fatalf("enqueues = %d; want 3 (the queue deduplicates, not the digest)", len(q.calls))
	}
	if q.calls[0].key != q.calls[1].key {
		t.Fatal("two runs in the same ISO week must share a dedup key")
	}
	if q.calls[0].key == q.calls[2].key {
		t.Fatal("the next ISO week must get its own dedup key")
	}
}

func TestDigestSearchesEveryKeywordInBothModes(t *testing.T) {
	t.Parallel()
	search := &fakeSearch{}
	q := &fakeEnqueuer{}
	if _, err := newDigestAt(digestNow, digestConn(true), search, q, "complejo de pádel", "reservar cancha").Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := []string{"complejo de pádel|TOP", "complejo de pádel|RECENT", "reservar cancha|TOP", "reservar cancha|RECENT"}
	if strings.Join(search.calls, ",") != strings.Join(want, ",") {
		t.Fatalf("searches = %v; want %v", search.calls, want)
	}
}

func TestDigestSurfacesASearchFailureAndMailsNothing(t *testing.T) {
	t.Parallel()
	search := &fakeSearch{err: errors.New("threads answered 500")}
	q := &fakeEnqueuer{}
	_, err := newDigestAt(digestNow, digestConn(true), search, q, "complejo de pádel").Run(context.Background())
	if err == nil {
		t.Fatal("a failed search must be returned so the job is retried, not swallowed")
	}
	if len(q.calls) != 0 {
		t.Fatal("a failed search must not mail a partial digest")
	}
}
