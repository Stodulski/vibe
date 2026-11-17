package notifications

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"
)

func monday() time.Weekday { return time.Monday }

func fullSeriesCancellation() SeriesCancellation {
	return SeriesCancellation{
		Email: "ana@example.com", ComplexName: "Vibe", CourtName: "Cancha 1",
		Weekday: time.Monday, Hours: "19:00 a 20:30", From: "14/12",
		BookURL: "https://vibe.test/vibe/book", SeriesID: "series-1",
	}
}

func TestSeriesCreatedIsEmailOnlyAndNeedsAnAddress(t *testing.T) {
	s, q, _, _ := newTestService(t)

	s.SeriesCreated(SeriesCreation{ComplexName: "Vibe", Weekday: monday(), SeriesID: "series-1"})
	if len(q.tasks) != 0 {
		t.Fatalf("no email address, nothing to send; got %v", q.types())
	}

	s.SeriesCreated(SeriesCreation{Email: "ana@example.com", Weekday: monday(), SeriesID: "series-1"})
	if got := q.types(); !slices.Equal(got, []string{TaskEmailSeriesCreated}) {
		t.Errorf("tasks = %v, want only the email", got)
	}
	if key, _ := q.keyOf(TaskEmailSeriesCreated); key == "" {
		t.Error("the creation email must be deduplicated")
	}
}

func TestSeriesCancelledQueuesTheEmailWithTheSeriesSentence(t *testing.T) {
	s, q, _, _ := newTestService(t)

	s.SeriesCancelled(fullSeriesCancellation())
	if got := q.types(); !slices.Equal(got, []string{TaskEmailSeriesCancelled}) {
		t.Fatalf("tasks = %v, want the email only", got)
	}
	if key, _ := q.keyOf(TaskEmailSeriesCancelled); key == "" {
		t.Error("the cancellation email must be deduplicated")
	}
}

func TestSeriesCancelledSkipsAClientWithNoEmail(t *testing.T) {
	s, q, _, _ := newTestService(t)
	c := fullSeriesCancellation()
	c.Email = ""
	s.SeriesCancelled(c)
	if len(q.tasks) != 0 {
		t.Errorf("no address, no task; got %v", q.types())
	}
}

func TestSeriesCancelledKeysDistinguishTheDateTheSeriesWasEndedFrom(t *testing.T) {
	s, q, _, _ := newTestService(t)
	s.SeriesCancelled(fullSeriesCancellation())
	later := fullSeriesCancellation()
	later.From = "21/12"
	s.SeriesCancelled(later)
	if q.tasks[0].dedupKey == "" || q.tasks[0].dedupKey == q.tasks[1].dedupKey {
		t.Errorf("keys %q / %q must be set and differ", q.tasks[0].dedupKey, q.tasks[1].dedupKey)
	}
}

func TestSeriesPayloadsCarryNoSeriesID(t *testing.T) {
	s, q, _, _ := newTestService(t)
	s.SeriesCreated(SeriesCreation{Email: "ana@example.com", SeriesID: "series-1"})
	s.SeriesCancelled(fullSeriesCancellation())
	for _, task := range q.tasks {
		raw, _ := json.Marshal(task.payload)
		if strings.Contains(string(raw), "series-1") {
			t.Errorf("%s payload stores the series id: %s", task.taskType, raw)
		}
	}
}

func TestSeriesWorkersRenderTheNoticesWithTheRightTemplate(t *testing.T) {
	s, q, mailer, _ := newTestService(t)
	s.RegisterWorkers()
	s.SeriesCreated(SeriesCreation{
		Email: "ana@example.com", ComplexName: "Vibe", CourtName: "Cancha 1", Weekday: time.Saturday,
		Hours: "19:00 a 20:30", From: "16/11", Until: "25/01",
	})
	s.SeriesCancelled(fullSeriesCancellation())

	for _, task := range q.tasks {
		raw, _ := json.Marshal(task.payload)
		if err := q.handlers[task.taskType](t.Context(), raw); err != nil {
			t.Fatalf("delivering %s: %v", task.taskType, err)
		}
	}
	if len(mailer.sent) != 2 || mailer.sent[0].kind != "series_created" || mailer.sent[1].kind != "series_cancelled" {
		t.Fatalf("mail sent = %+v", mailer.sent)
	}
	created := mailer.sent[0].args
	if created[2] != "sábados" || created[3] != "19:00 a 20:30" || created[1] != "Cancha 1" || created[4] != "16/11" || created[5] != "25/01" {
		t.Errorf("created args = %v", created)
	}
}

func TestSpanishWeekdays(t *testing.T) {
	want := map[time.Weekday][2]string{
		time.Monday: {"lunes", "lunes"}, time.Wednesday: {"miércoles", "miércoles"},
		time.Saturday: {"sábado", "sábados"}, time.Sunday: {"domingo", "domingos"},
	}
	for d, w := range want {
		if SpanishWeekday(d) != w[0] || SpanishWeekdays(d) != w[1] {
			t.Errorf("%v = %s / %s, want %v", d, SpanishWeekday(d), SpanishWeekdays(d), w)
		}
	}
}
