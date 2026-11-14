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
		Email: "ana@example.com", Phone: "+5411", ComplexName: "Vibe", CourtName: "Cancha 1",
		Weekday: time.Monday, Hours: "19:00 a 20:30", From: "14/12",
		BookPath: "vibe/book", BookURL: "https://vibe.test/vibe/book", SeriesID: "series-1",
	}
}

func TestSeriesCreatedIsEmailOnlyAndNeedsAnAddress(t *testing.T) {
	s, q, _, _, _ := newTestService(t, true)

	s.SeriesCreated(SeriesCreation{ComplexName: "Vibe", Weekday: monday(), SeriesID: "series-1"})
	if len(q.tasks) != 0 {
		t.Fatalf("no email address, nothing to send; got %v", q.types())
	}

	s.SeriesCreated(SeriesCreation{Email: "ana@example.com", Weekday: monday(), SeriesID: "series-1"})
	if got := q.types(); !slices.Equal(got, []string{TaskEmailSeriesCreated}) {
		t.Errorf("tasks = %v, want only the email (never WhatsApp for creation)", got)
	}
	if key, _ := q.keyOf(TaskEmailSeriesCreated); key == "" {
		t.Error("the creation email must be deduplicated")
	}
}

func TestSeriesCancelledQueuesWhatsAppWithTheApprovedTemplateValuesAndEmail(t *testing.T) {
	s, q, _, _, _ := newTestService(t, true)

	s.SeriesCancelled(fullSeriesCancellation())
	if got := q.types(); !slices.Equal(got, []string{TaskEmailSeriesCancelled, TaskWASeriesCancelled}) {
		t.Fatalf("tasks = %v", got)
	}
	wa := q.tasks[1].payload.(waBookingCancelled)
	if wa.Date != "lunes 14/12 y los lunes siguientes" || wa.StartTime != "19:00 a 20:30" ||
		wa.RefundLine != "no hay pagos a devolver." || wa.BookPath != "vibe/book" || wa.CourtName != "Cancha 1" {
		t.Errorf("whatsapp payload = %+v", wa)
	}

	paid := fullSeriesCancellation()
	paid.KeptPaid, paid.Weekday = true, time.Saturday
	q.tasks = nil
	s.SeriesCancelled(paid)
	wa = q.tasks[1].payload.(waBookingCancelled)
	if wa.RefundLine != "los turnos ya pagos se mantienen." || wa.Date != "sábado 14/12 y los sábados siguientes" {
		t.Errorf("whatsapp payload = %+v", wa)
	}
}

func TestSeriesCancelledSkipsWhatsAppWhenNotConfiguredOrWithoutPhone(t *testing.T) {
	s, q, _, _, _ := newTestService(t, false)
	s.SeriesCancelled(fullSeriesCancellation())
	if q.has(TaskWASeriesCancelled) || !q.has(TaskEmailSeriesCancelled) {
		t.Errorf("tasks = %v, want the email only", q.types())
	}

	s, q, _, _, _ = newTestService(t, true)
	c := fullSeriesCancellation()
	c.Phone, c.Email = "", ""
	s.SeriesCancelled(c)
	if len(q.tasks) != 0 {
		t.Errorf("no channel, no task; got %v", q.types())
	}
}

func TestSeriesCancelledKeysDistinguishTheDateTheSeriesWasEndedFrom(t *testing.T) {
	s, q, _, _, _ := newTestService(t, true)
	s.SeriesCancelled(fullSeriesCancellation())
	later := fullSeriesCancellation()
	later.From = "21/12"
	s.SeriesCancelled(later)
	if q.tasks[0].dedupKey == "" || q.tasks[0].dedupKey == q.tasks[2].dedupKey {
		t.Errorf("keys %q / %q must be set and differ", q.tasks[0].dedupKey, q.tasks[2].dedupKey)
	}
}

func TestSeriesPayloadsCarryNoSeriesID(t *testing.T) {
	s, q, _, _, _ := newTestService(t, true)
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
	s, q, mailer, wa, _ := newTestService(t, true)
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
	if len(wa.sent) != 1 || wa.sent[0].tmpl.TemplateName != "booking_cancelled" || wa.sent[0].to != "+5411" {
		t.Errorf("whatsapp sent = %+v", wa.sent)
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
