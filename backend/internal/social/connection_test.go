package social

import (
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"strings"
	"testing"
)

const connectionToken = "connection-token-value"

type fakeChecker struct {
	report ConnectionReport
	calls  int
}

func (f *fakeChecker) CheckConnection(context.Context) ConnectionReport {
	f.calls++
	return f.report
}

type recordedEnqueue struct {
	taskType string
	payload  ReportPayload
	key      string
}

type fakeEnqueuer struct{ calls []recordedEnqueue }

func (f *fakeEnqueuer) Enqueue(taskType string, payload any, dedupKey string) {
	p, ok := payload.(ReportPayload)
	if !ok {
		panic("connection email must be a ReportPayload")
	}
	f.calls = append(f.calls, recordedEnqueue{taskType: taskType, payload: p, key: dedupKey})
}

func healthyReport() ConnectionReport {
	return ConnectionReport{PageName: "Vibe", PageID: "page-1", IGUsername: "vibe.padel", IGUserID: "ig-1"}
}

func runConnector(t *testing.T, checker *fakeChecker, token string) (*fakeEnqueuer, bool) {
	t.Helper()
	queue := &fakeEnqueuer{}
	c := NewConnector(checker, queue, "hola@vibe.com.ar", token, slog.New(slog.DiscardHandler))
	_, ran := c.Run(context.Background())
	return queue, ran
}

func TestConnectorQueuesOneOKEmailAndNeverCarriesTheToken(t *testing.T) {
	t.Parallel()
	queue, ran := runConnector(t, &fakeChecker{report: healthyReport()}, connectionToken)
	if !ran || len(queue.calls) != 1 {
		t.Fatalf("ran=%v enqueued=%d, want one email", ran, len(queue.calls))
	}
	call := queue.calls[0]
	if call.taskType != TaskConnection || call.payload.Subject != "Conexión con Meta: OK" || call.payload.To != "hola@vibe.com.ar" {
		t.Fatalf("email = %+v, want the OK subject to the report address", call)
	}
	if !slices.Contains(call.payload.Lines, "Página: Vibe (page-1).") || !slices.Contains(call.payload.Lines, "Instagram: @vibe.padel (ig-1).") {
		t.Fatalf("lines = %v, want the page and the Instagram account", call.payload.Lines)
	}
	raw, err := json.Marshal(call.payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if strings.Contains(string(raw), connectionToken) || strings.Contains(call.key, connectionToken) {
		t.Fatal("the connection email and its dedup key must never carry the token")
	}
}

func TestConnectorSubjectNamesProblemsAndListsThem(t *testing.T) {
	t.Parallel()
	report := healthyReport()
	report.Missing = []string{"pages_manage_posts"}
	report.Problems = []string{"el token ve 2 páginas; configurá META_SOCIAL_PAGE_ID"}
	queue, _ := runConnector(t, &fakeChecker{report: report}, connectionToken)

	call := queue.calls[0]
	if call.payload.Subject != "Conexión con Meta: con problemas" {
		t.Fatalf("subject = %q, want the problems subject", call.payload.Subject)
	}
	body := strings.Join(call.payload.Lines, "\n")
	for _, want := range []string{"pages_manage_posts", "configurá META_SOCIAL_PAGE_ID"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body %q is missing %q", body, want)
		}
	}
}

func TestConnectorDedupKeyIsStablePerTokenAndResult(t *testing.T) {
	t.Parallel()
	first, _ := runConnector(t, &fakeChecker{report: healthyReport()}, connectionToken)
	again, _ := runConnector(t, &fakeChecker{report: healthyReport()}, connectionToken)
	if first.calls[0].key != again.calls[0].key {
		t.Fatal("the same token and the same result must produce the same dedup key, so one email is sent")
	}

	otherResult := healthyReport()
	otherResult.IGUsername = "other"
	changed, _ := runConnector(t, &fakeChecker{report: otherResult}, connectionToken)
	if changed.calls[0].key == first.calls[0].key {
		t.Fatal("a different result must produce a new dedup key")
	}

	otherToken, _ := runConnector(t, &fakeChecker{report: healthyReport()}, "another-token")
	if otherToken.calls[0].key == first.calls[0].key {
		t.Fatal("a different token must produce a new dedup key")
	}
}

func TestConnectorDoesNothingWithoutAToken(t *testing.T) {
	t.Parallel()
	checker := &fakeChecker{report: healthyReport()}
	queue, ran := runConnector(t, checker, "")
	if ran || checker.calls != 0 || len(queue.calls) != 0 {
		t.Fatalf("ran=%v checks=%d emails=%d; want nothing without a token", ran, checker.calls, len(queue.calls))
	}
}

func TestConnectionReportOKNeedsNoProblemsAndNoMissingPermissions(t *testing.T) {
	t.Parallel()
	if !healthyReport().OK() {
		t.Fatal("a healthy report must be OK")
	}
	missing := healthyReport()
	missing.Missing = []string{"instagram_basic"}
	if missing.OK() {
		t.Fatal("a missing permission must not be OK")
	}
}
