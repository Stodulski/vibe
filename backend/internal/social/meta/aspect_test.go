package meta

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/social"
)

// probeGraph answers the probe's calls: it creates a container, reports its
// status, and records any publish attempt, which the probe must never make.
type probeGraph struct {
	mu        sync.Mutex
	status    string
	errText   string
	published int
	created   []string
}

func (g *probeGraph) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch {
	case strings.HasSuffix(r.URL.Path, "/media_publish"):
		g.published++
		_, _ = w.Write([]byte(`{"id":"should-not-happen"}`))
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/media"):
		g.created = append(g.created, r.FormValue("image_url"))
		_, _ = w.Write([]byte(`{"id":"probe-1"}`))
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/probe-1"):
		_ = json.NewEncoder(w).Encode(map[string]string{"status_code": g.status, "status": g.errText})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func newProbeClient(t *testing.T, g *probeGraph) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(g)
	t.Cleanup(srv.Close)
	c := New(Config{Token: "system-token", APIBase: srv.URL, PollEvery: time.Millisecond, PollFor: time.Second})
	return c, srv
}

func TestAspectProbeReportsAcceptanceWithoutPublishing(t *testing.T) {
	t.Parallel()
	g := &probeGraph{status: "FINISHED"}
	c, _ := newProbeClient(t, g)
	probe := NewAspectProbe(c, "https://cdn.test/social/probe/3x4.jpg", slog.New(slog.DiscardHandler))

	line := probe.Check(context.Background(), social.ConnectionReport{IGUserID: "ig-1"})
	if line.Text != "Formato 3:4: aceptado." || line.Problem {
		t.Fatalf("line = %+v; want accepted and no problem", line)
	}
	if g.published != 0 {
		t.Fatal("the probe must never publish")
	}
	if len(g.created) != 1 || g.created[0] != "https://cdn.test/social/probe/3x4.jpg" {
		t.Fatalf("created %v; want one container for the probe image", g.created)
	}
}

func TestAspectProbeReportsRejectionWithMetasReason(t *testing.T) {
	t.Parallel()
	g := &probeGraph{status: "ERROR", errText: "Aspect ratio not supported"}
	c, _ := newProbeClient(t, g)
	probe := NewAspectProbe(c, "https://cdn.test/social/probe/3x4.jpg", slog.New(slog.DiscardHandler))

	line := probe.Check(context.Background(), social.ConnectionReport{IGUserID: "ig-1"})
	if !strings.Contains(line.Text, "rechazado: Aspect ratio not supported") || line.Problem {
		t.Fatalf("line = %+v; want rejected with Meta's reason, informational only", line)
	}
	if g.published != 0 {
		t.Fatal("a rejected probe must not publish either")
	}
}

func TestAspectProbeWithoutAnAccountOrImageSaysItDidNotRun(t *testing.T) {
	t.Parallel()
	g := &probeGraph{status: "FINISHED"}
	c, _ := newProbeClient(t, g)

	noAccount := NewAspectProbe(c, "https://cdn.test/x.jpg", slog.New(slog.DiscardHandler))
	if line := noAccount.Check(context.Background(), social.ConnectionReport{}); !strings.Contains(line.Text, "no probado") {
		t.Fatalf("without an Instagram account the probe must say so; got %q", line.Text)
	}
	noImage := NewAspectProbe(c, "", slog.New(slog.DiscardHandler))
	if line := noImage.Check(context.Background(), social.ConnectionReport{IGUserID: "ig-1"}); !strings.Contains(line.Text, "no probado") {
		t.Fatalf("without a probe image the probe must say so; got %q", line.Text)
	}
	if len(g.created) != 0 {
		t.Fatal("neither case may create a container")
	}
}
