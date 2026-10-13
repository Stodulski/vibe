package social

import (
	"bytes"
	"context"
	"errors"
	"image/jpeg"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/storage"
)

func TestThreadsCheckDescribesEachConnectionState(t *testing.T) {
	t.Parallel()
	now := func() time.Time { return t0 }
	cases := []struct {
		name       string
		configured bool
		store      *fakeThreadsStore
		want       string
		problem    bool
	}{
		{name: "app not configured", configured: false, store: &fakeThreadsStore{}, want: "no está configurada"},
		{name: "not connected", configured: true, store: &fakeThreadsStore{}, want: "no conectado"},
		{
			name: "connected", configured: true, want: "@vibe.padel conectado, el token vence en 10 días",
			store: storedConn(5*24*time.Hour, time.Hour, 10*24*time.Hour),
		},
		{
			name: "expired", configured: true, want: "venció", problem: true,
			store: storedConn(80*24*time.Hour, 60*24*time.Hour, -time.Hour),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			check := threadsCheck{store: tc.store, configured: tc.configured, connectURL: connectURL, now: now}
			line := check.Check(context.Background(), ConnectionReport{})
			if !strings.Contains(line.Text, tc.want) || line.Problem != tc.problem {
				t.Fatalf("line = %+v; want text containing %q, problem %v", line, tc.want, tc.problem)
			}
			if strings.Contains(line.Text, "long-old") {
				t.Fatal("the status line must never carry the token")
			}
		})
	}
}

func TestConnectorAppendsItsChecksToTheEmail(t *testing.T) {
	t.Parallel()
	q := &fakeEnqueuer{}
	checker := &fakeChecker{report: healthyReport()}
	extra := fixedCheck{line: ReportLine{Text: "Formato 3:4: aceptado."}}
	ok := fixedCheck{line: ReportLine{Text: "Threads: token vencido.", Problem: true}}
	c := NewConnector(checker, q, "owner@vibe.test", "token-1", slog.New(slog.DiscardHandler)).WithChecks(extra, ok)

	c.Run(context.Background())
	if len(q.calls) != 1 {
		t.Fatalf("calls = %d; want one email", len(q.calls))
	}
	p := q.calls[0].payload
	body := strings.Join(p.Lines, "\n")
	if !strings.Contains(body, "Formato 3:4: aceptado.") || !strings.Contains(body, "Threads: token vencido.") {
		t.Fatalf("lines = %q; want both checks", body)
	}
	if !strings.Contains(p.Subject, "con problemas") {
		t.Fatalf("subject = %q; a problem line must make the verdict a problem", p.Subject)
	}
}

func TestProbeImageIsA1080By1440JPEG(t *testing.T) {
	t.Parallel()
	raw, err := ProbeImage()
	if err != nil {
		t.Fatalf("probe image: %v", err)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("probe image is not a JPEG: %v", err)
	}
	if cfg.Width != 1080 || cfg.Height != 1440 {
		t.Fatalf("size = %dx%d; want 1080x1440 (3:4)", cfg.Width, cfg.Height)
	}
}

type fixedCheck struct{ line ReportLine }

func (f fixedCheck) Check(context.Context, ConnectionReport) ReportLine { return f.line }

type probeMedia struct {
	keys    []string
	types   []string
	putErr  error
	baseURL string
}

func (m *probeMedia) PutObject(_ context.Context, obj storage.Object) error {
	m.keys = append(m.keys, obj.Key)
	m.types = append(m.types, obj.ContentType)
	return m.putErr
}

func (m *probeMedia) PublicURL(key string) string {
	if m.baseURL == "" {
		return ""
	}
	return m.baseURL + "/" + key
}

func TestEnsureProbeImageUploadsUnderTheProbeKey(t *testing.T) {
	t.Parallel()
	media := &probeMedia{baseURL: "https://cdn.test"}
	url, err := EnsureProbeImage(context.Background(), media)
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if url != "https://cdn.test/social/probe/3x4.jpg" {
		t.Fatalf("url = %q", url)
	}
	if len(media.keys) != 1 || media.keys[0] != ProbeImageKey || media.types[0] != "image/jpeg" {
		t.Fatalf("uploads = %v %v; want one JPEG under the probe key", media.keys, media.types)
	}
}

func TestEnsureProbeImageFailsWithoutAPublicURL(t *testing.T) {
	t.Parallel()
	if _, err := EnsureProbeImage(context.Background(), &probeMedia{putErr: errors.New("boom")}); err == nil {
		t.Fatal("a failed upload must be an error")
	}
	if _, err := EnsureProbeImage(context.Background(), &probeMedia{}); err == nil {
		t.Fatal("no public URL means the probe cannot run")
	}
}
