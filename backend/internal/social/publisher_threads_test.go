package social

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func threadsTextDue(id string, networks ...Network) Post {
	at := t0.Add(-time.Minute)
	return Post{
		ID: id, PublishAt: at, Networks: networks, Kind: TypeText, Caption: "Solo texto",
		Status: StatusScheduled, NextAttemptAt: &at,
	}
}

func TestThreadsTextPostPublishesAndReportsItsLink(t *testing.T) {
	t.Parallel()
	h := newHarness(t, true, threadsTextDue("texto", NetworkThreads))
	h.platform.thLink = "https://www.threads.net/@vibe.padel/post/abc"

	if _, err := h.pub.RunDue(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	got := h.post("texto")
	if got.Status != StatusPublished || got.ThreadsMediaID != "th-1" {
		t.Fatalf("post = %+v; want published with the threads id", got)
	}
	if h.platform.thKind != TypeText {
		t.Fatalf("threads received kind %q; want text", h.platform.thKind)
	}
	if len(h.reporter.reports) != 1 || h.reporter.reports[0].ThreadsLink != h.platform.thLink {
		t.Fatalf("reports = %+v; want one report carrying the threads link", h.reporter.reports)
	}
}

func TestPartialThreadsFailureRetriesOnlyThreads(t *testing.T) {
	t.Parallel()
	post := duePost("parcial", NetworkInstagram, NetworkThreads)
	h := newHarness(t, true, post)
	h.platform.thErr = errors.New("threads container ended as ERROR")

	if _, err := h.pub.RunDue(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	after := h.post("parcial")
	if after.Status != StatusPartial || after.IGMediaID != "ig-1" || after.ThreadsMediaID != "" {
		t.Fatalf("after the first run = %+v; want partial with instagram done and threads missing", after)
	}
	if !strings.Contains(after.LastError, "threads") {
		t.Fatalf("last error = %q; want the threads failure", after.LastError)
	}

	h.platform.thErr = nil
	h.now = t0.Add(10 * time.Minute)
	if _, err := h.pub.RunDue(context.Background()); err != nil {
		t.Fatalf("second run: %v", err)
	}
	done := h.post("parcial")
	if done.Status != StatusPublished || done.ThreadsMediaID != "th-1" {
		t.Fatalf("after the retry = %+v; want published", done)
	}
	if h.platform.igCalls != 1 {
		t.Fatalf("instagram was published %d times; a retry must not repeat it", h.platform.igCalls)
	}
	if h.platform.thCalls != 2 {
		t.Fatalf("threads was tried %d times; want the failed attempt and the retry", h.platform.thCalls)
	}
}

func TestThreadsPermalinkIsOmittedWhenItCannotBeRead(t *testing.T) {
	t.Parallel()
	p := threadsTextDue("sin-enlace", NetworkThreads)
	p.ThreadsMediaID = "th-9"
	rep := BuildReport("owner@vibe.test", Report{Post: Post{ID: "sin-enlace", Status: StatusPublished, Networks: []Network{NetworkThreads}, ThreadsMediaID: "th-9"}})
	joined := strings.Join(rep.Lines, "\n")
	if !strings.Contains(joined, "Threads: publicada, sin enlace.") {
		t.Fatalf("report lines = %q; want the threads line without a link", joined)
	}
}
