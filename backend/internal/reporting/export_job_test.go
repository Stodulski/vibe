package reporting

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/jobs"
	"github.com/stodulski/vibe-server/internal/storage"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// stubExportStore is the jobs table as this module sees it, in memory. It
// records every Enqueue so a test can tell "the same export came back" from
// "a second one was queued", which is the property the deduplication exists
// for and the one a count cannot be inferred from an id alone.
type stubExportStore struct {
	byID  map[uuid.UUID]*jobs.Job
	byKey map[string]uuid.UUID

	enqueued int
	err      error
}

func newStubExportStore() *stubExportStore {
	return &stubExportStore{byID: map[uuid.UUID]*jobs.Job{}, byKey: map[string]uuid.UUID{}}
}

func (s *stubExportStore) Enqueue(_ context.Context, jobType string, payload any, _ time.Time, _ int, dedupKey string) (uuid.UUID, bool, error) {
	if s.err != nil {
		return uuid.Nil, false, s.err
	}
	if dedupKey != "" {
		if _, taken := s.byKey[dedupKey]; taken {
			return uuid.Nil, false, nil
		}
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return uuid.Nil, false, err
	}
	id := uuid.New()
	now := time.Now()
	s.byID[id] = &jobs.Job{
		ID: id, Type: jobType, Payload: raw, Status: jobs.StatusPending,
		DedupKey: dedupKey, CreatedAt: now, UpdatedAt: now,
	}
	if dedupKey != "" {
		s.byKey[dedupKey] = id
	}
	s.enqueued++
	return id, true, nil
}

func (s *stubExportStore) GetByDedupKey(_ context.Context, key string) (*jobs.Job, error) {
	id, ok := s.byKey[key]
	if !ok {
		return nil, data.ErrRecordNotFound
	}
	return s.byID[id], nil
}

func (s *stubExportStore) ReleaseDedupKey(_ context.Context, id uuid.UUID) (bool, error) {
	job, ok := s.byID[id]
	if !ok || job.Status != jobs.StatusFailed || job.DedupKey == "" {
		return false, nil
	}
	delete(s.byKey, job.DedupKey)
	job.DedupKey = ""
	return true, nil
}

func (s *stubExportStore) GetExport(_ context.Context, id uuid.UUID, complexID, jobType string) (*jobs.Job, error) {
	job, ok := s.byID[id]
	if !ok || job.Type != jobType {
		return nil, data.ErrRecordNotFound
	}
	var p ExportPaymentsPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil || p.ComplexID != complexID {
		return nil, data.ErrRecordNotFound
	}
	return job, nil
}

// stubExportStorage is the private bucket. It records what was written so the
// worker tests can assert the key, type and disposition, and signs a URL that
// carries the arguments it was given so a handler test can read them back.
type stubExportStorage struct {
	put []storage.Object
	err error

	signedKey  string
	signedName string
	signedTTL  time.Duration
}

func (s *stubExportStorage) PutObject(_ context.Context, obj storage.Object) error {
	if s.err != nil {
		return s.err
	}
	s.put = append(s.put, obj)
	return nil
}

func (s *stubExportStorage) GeneratePresignedGET(_ context.Context, key, downloadName string, ttl time.Duration) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	s.signedKey, s.signedName, s.signedTTL = key, downloadName, ttl
	return "https://private.example/" + key + "?signed", nil
}

// exportFixture is a handler wired to an in-memory queue and bucket, plus the
// complex every request below is made for.
type exportFixture struct {
	handler *Handler
	svc     *Service
	store   *stubExportStore
	bucket  *stubExportStorage
	complex *complexstore.Complex
}

func newExportFixture(t *testing.T) *exportFixture {
	t.Helper()

	store := newStubExportStore()
	bucket := &stubExportStorage{}
	svc := NewService(stubBookings{}, stubClients{}, stubCourts{}, stubSchedules{}, &stubReports{},
		ExportDeps{Store: store, Storage: bucket}, 50*time.Second)

	return &exportFixture{
		handler: newTestHandlerWithService(svc),
		svc:     svc,
		store:   store,
		bucket:  bucket,
		complex: &complexstore.Complex{
			ID:        uuid.New(),
			Name:      "Vibe Palermo",
			Slug:      "vibe-palermo",
			CreatedAt: time.Date(2020, 1, 1, 0, 0, 0, 0, timezone.Argentina),
		},
	}
}

// post issues a create request carrying the complex the ownership middleware
// would have put in context. An empty body means "the current month", which
// is the ordinary request.
func (f *exportFixture) post(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()

	target := "/api/v1/complexes/" + f.complex.ID.String() + "/reports/exports"
	var r *http.Request
	if body == "" {
		r = httptest.NewRequestWithContext(t.Context(), http.MethodPost, target, nil)
	} else {
		r = httptest.NewRequestWithContext(t.Context(), http.MethodPost, target, strings.NewReader(body))
	}
	w := httptest.NewRecorder()
	f.handler.CreatePaymentsExport(w, httpx.ContextSetComplex(r, f.complex))
	return w
}

// get issues a status request for one export id.
func (f *exportFixture) get(t *testing.T, exportID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		exportStatusPath(f.complex.ID, exportID), nil)
	r.SetPathValue("exportID", exportID.String())
	w := httptest.NewRecorder()
	f.handler.GetPaymentsExport(w, httpx.ContextSetComplex(r, f.complex))
	return w
}

// decodeExport reads the `export` object out of a response body.
func decodeExport(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var body struct {
		Export map[string]any `json:"export"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding the response (%s): %v", w.Body.String(), err)
	}
	return body.Export
}

func TestCreatingAnExportAnswers202WithSomewhereToPoll(t *testing.T) {
	f := newExportFixture(t)

	w := f.post(t, "")
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body: %s", w.Code, w.Body.String())
	}

	export := decodeExport(t, w)
	id, _ := export["id"].(string)
	if _, err := uuid.Parse(id); err != nil {
		t.Errorf("the export id %q is not a uuid the client can poll with", id)
	}
	if got := export["status"]; got != exportStatusPending {
		t.Errorf("status = %v, want %q: nothing has claimed the job yet", got, exportStatusPending)
	}

	want := exportStatusPath(f.complex.ID, uuid.MustParse(id))
	if got := export["status_url"]; got != want {
		t.Errorf("status_url = %v, want %q", got, want)
	}
	if got := w.Header().Get("Location"); got != want {
		t.Errorf("Location = %q, want %q — a 202 has to say where the thing it accepted lives", got, want)
	}
}

// TestCreatingAnExportDefaultsThePeriodForAnEmptyChunkedBody proves the
// period still defaults to the current month when the body arrives with no
// Content-Length at all — as a chunked-encoded request does — and turns out
// to hold zero bytes once read. r.ContentLength == 0 alone (the check
// readExportPeriod used to make) never sees this case: net/http reports -1
// for a chunked request's ContentLength whether or not the body is empty.
func TestCreatingAnExportDefaultsThePeriodForAnEmptyChunkedBody(t *testing.T) {
	f := newExportFixture(t)

	target := "/api/v1/complexes/" + f.complex.ID.String() + "/reports/exports"
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, target, strings.NewReader(""))
	r.ContentLength = -1 // what a chunked request carries, empty or not.

	w := httptest.NewRecorder()
	f.handler.CreatePaymentsExport(w, httpx.ContextSetComplex(r, f.complex))

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body: %s", w.Code, w.Body.String())
	}

	export := decodeExport(t, w)
	job := f.store.byID[uuid.MustParse(export["id"].(string))]
	var payload ExportPaymentsPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		t.Fatalf("decoding the queued payload: %v", err)
	}

	now := time.Now().In(timezone.Argentina)
	if payload.Month != int(now.Month()) || payload.Year != now.Year() {
		t.Errorf("queued period = %d/%d, want the current month %d/%d",
			payload.Month, payload.Year, int(now.Month()), now.Year())
	}
}

// The whole point of the dedup key: an owner who clicks twice gets the export
// that is already running, not a second one building the same month.
func TestASecondExportForTheSameDayReturnsTheFirst(t *testing.T) {
	f := newExportFixture(t)

	first := decodeExport(t, f.post(t, ""))
	second := decodeExport(t, f.post(t, ""))

	if first["id"] != second["id"] {
		t.Errorf("the second request got export %v, want the first one %v", second["id"], first["id"])
	}
	if f.store.enqueued != 1 {
		t.Errorf("%d jobs were queued for one period and one day; want 1", f.store.enqueued)
	}
}

// A dead-lettered export is work nobody is doing any more, so asking again has
// to queue it again — and the dead row must survive as the record of what
// failed.
func TestRetryingAFailedExportQueuesANewOne(t *testing.T) {
	f := newExportFixture(t)

	firstID := uuid.MustParse(decodeExport(t, f.post(t, ""))["id"].(string))
	f.store.byID[firstID].Status = jobs.StatusFailed

	second := decodeExport(t, f.post(t, ""))
	if second["id"] == firstID.String() {
		t.Fatal("the retry answered with the dead export's id; nothing is going to produce that file")
	}
	if f.store.enqueued != 2 {
		t.Errorf("%d jobs queued; want 2 — the failed one and its retry", f.store.enqueued)
	}
	if f.store.byID[firstID].Status != jobs.StatusFailed {
		t.Error("the dead letter was reused rather than left standing; it is the only record that this failed")
	}
}

func TestAPeriodTheOwnerCannotExportIsRefusedBeforeAnythingIsQueued(t *testing.T) {
	f := newExportFixture(t)

	next := time.Now().In(timezone.Argentina).AddDate(0, 1, 0)
	body, err := json.Marshal(map[string]int{"month": int(next.Month()), "year": next.Year()})
	if err != nil {
		t.Fatal(err)
	}

	w := f.post(t, string(body))
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for a period that has not happened yet", w.Code)
	}
	if f.store.enqueued != 0 {
		t.Error("a refused period still queued work")
	}
}

// The tenant predicate lives in the store; this is the answer it produces at
// the edge. A 404 rather than a 403, so the route cannot be used to discover
// which export ids exist under another complex.
func TestAForeignExportIdIsNotFound(t *testing.T) {
	f := newExportFixture(t)
	id := uuid.MustParse(decodeExport(t, f.post(t, ""))["id"].(string))

	other := *f.complex
	other.ID = uuid.New()
	f.complex = &other

	if w := f.get(t, id); w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for another complex's export", w.Code)
	}
	if w := f.get(t, uuid.New()); w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for an export that does not exist", w.Code)
	}
}

// A failed export is a successful status read of a failed thing. Answering
// 4xx here would make a polling client treat its own request as broken.
func TestAFailedExportIs200WithAnEmbeddedProblem(t *testing.T) {
	f := newExportFixture(t)
	id := uuid.MustParse(decodeExport(t, f.post(t, ""))["id"].(string))

	f.store.byID[id].Status = jobs.StatusFailed
	f.store.byID[id].LastError = httpx.CodeExportTooLarge + ": jobs: job permanently rejected"

	w := f.get(t, id)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	export := decodeExport(t, w)
	if export["status"] != exportStatusFailed {
		t.Errorf("status = %v, want %q", export["status"], exportStatusFailed)
	}

	problem, ok := export["error"].(map[string]any)
	if !ok {
		t.Fatalf("a failed export carries no embedded problem: %s", w.Body.String())
	}
	if problem["detail"] != httpx.CodeExportTooLarge {
		t.Errorf("detail = %v, want the machine code %q", problem["detail"], httpx.CodeExportTooLarge)
	}
	if want := exportStatusPath(f.complex.ID, id); problem["instance"] != want {
		t.Errorf("instance = %v, want %q — the problem is about this export, not about the collection",
			problem["instance"], want)
	}
	if problem["status"] != float64(http.StatusUnprocessableEntity) {
		t.Errorf("status = %v, want 422: the same status the synchronous export refuses an oversized period with",
			problem["status"])
	}
}

// Anything that is not a refusal the owner can act on collapses to one code:
// last_error is a Go error message and publishing it hands a tenant our
// internals.
func TestAnUnrecognizedFailureDoesNotLeakTheJobsError(t *testing.T) {
	f := newExportFixture(t)
	id := uuid.MustParse(decodeExport(t, f.post(t, ""))["id"].(string))

	f.store.byID[id].Status = jobs.StatusFailed
	f.store.byID[id].LastError = `pq: relation "payments" does not exist`

	problem, ok := decodeExport(t, f.get(t, id))["error"].(map[string]any)
	if !ok {
		t.Fatal("a failed export carries no embedded problem")
	}
	if problem["detail"] != httpx.CodeExportFailed {
		t.Errorf("detail = %v, want %q", problem["detail"], httpx.CodeExportFailed)
	}
	rendered, err := json.Marshal(problem)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(rendered), "relation") {
		t.Errorf("the job's raw error reached the client: %s", rendered)
	}
}

func TestAFinishedExportIsSignedForFifteenMinutes(t *testing.T) {
	f := newExportFixture(t)
	id := uuid.MustParse(decodeExport(t, f.post(t, ""))["id"].(string))

	job := f.store.byID[id]
	job.Status = jobs.StatusDone
	job.UpdatedAt = time.Now()

	export := decodeExport(t, f.get(t, id))
	if export["status"] != exportStatusDone {
		t.Fatalf("status = %v, want done", export["status"])
	}
	if export["download_url"] == nil || export["expires_at"] == nil {
		t.Fatalf("a finished export must carry a download URL and its expiry: %v", export)
	}

	var payload ExportPaymentsPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if f.bucket.signedKey != exportObjectKey(payload) {
		t.Errorf("signed key = %q, want %q", f.bucket.signedKey, exportObjectKey(payload))
	}
	if f.bucket.signedName != exportFilename(payload) {
		t.Errorf("signed download name = %q, want %q — the owner saves this file under it",
			f.bucket.signedName, exportFilename(payload))
	}
	if f.bucket.signedTTL != exportDownloadTTL {
		t.Errorf("ttl = %s, want %s", f.bucket.signedTTL, exportDownloadTTL)
	}
}

// The jobs table keeps a finished row for seven days and the bucket's
// lifecycle rule removes the file after one. Without this the status route
// would sign a URL for an object that is already gone.
func TestAnExportPastItsRetentionIsGone(t *testing.T) {
	f := newExportFixture(t)
	id := uuid.MustParse(decodeExport(t, f.post(t, ""))["id"].(string))

	job := f.store.byID[id]
	job.Status = jobs.StatusDone
	job.UpdatedAt = time.Now().Add(-exportLifetime - time.Minute)

	res := f.get(t, id)
	if res.Code != http.StatusGone {
		t.Fatalf("status = %d, want 410 for an export whose file the lifecycle rule collected", res.Code)
	}
	if f.bucket.signedKey != "" {
		t.Error("a URL was signed for an object that no longer exists")
	}

	var problem httpx.Problem
	if err := json.Unmarshal(res.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decoding the problem: %v", err)
	}
	if problem.Detail != httpx.CodeExportExpired {
		t.Errorf("detail = %q, want %q", problem.Detail, httpx.CodeExportExpired)
	}
}

// A deployment with no private bucket cannot export at all, and says so the
// same way the upload endpoints do rather than failing halfway through.
func TestExportsAnswer501WhenNoPrivateStorageIsConfigured(t *testing.T) {
	svc := NewService(stubBookings{}, stubClients{}, stubCourts{}, stubSchedules{}, &stubReports{},
		ExportDeps{}, 50*time.Second)
	f := &exportFixture{
		handler: newTestHandlerWithService(svc),
		svc:     svc,
		store:   newStubExportStore(),
		bucket:  &stubExportStorage{},
		complex: &complexstore.Complex{
			ID: uuid.New(), Name: "Vibe Palermo", Slug: "vibe-palermo",
			CreatedAt: time.Date(2020, 1, 1, 0, 0, 0, 0, timezone.Argentina),
		},
	}

	for _, res := range []*httptest.ResponseRecorder{f.post(t, ""), f.get(t, uuid.New())} {
		if res.Code != http.StatusNotImplemented {
			t.Errorf("status = %d, want 501", res.Code)
		}
		if ct := res.Header().Get("Content-Type"); ct != "application/problem+json" {
			t.Errorf("Content-Type = %q, want application/problem+json", ct)
		}

		var problem httpx.Problem
		if err := json.Unmarshal(res.Body.Bytes(), &problem); err != nil {
			t.Fatalf("decoding the problem: %v", err)
		}
		if want := httpx.KindUnavailable.URI(); problem.Type != want {
			t.Errorf("problem type = %q, want %q", problem.Type, want)
		}
	}
}

// The queue is not a bucket: without either half the feature does not exist,
// and half-configured must not look configured.
func TestExportsNeedBothTheQueueAndTheBucket(t *testing.T) {
	cases := map[string]ExportDeps{
		"neither":      {},
		"queue only":   {Store: newStubExportStore()},
		"bucket only":  {Storage: &stubExportStorage{}},
		"both present": {Store: newStubExportStore(), Storage: &stubExportStorage{}},
	}

	for name, deps := range cases {
		t.Run(name, func(t *testing.T) {
			svc := NewService(stubBookings{}, stubClients{}, stubCourts{}, stubSchedules{},
				&stubReports{}, deps, 50*time.Second)
			want := name == "both present"
			if got := svc.ExportsConfigured(); got != want {
				t.Errorf("ExportsConfigured() = %v, want %v", got, want)
			}
		})
	}
}

// The key identifies one piece of work, and the day is part of it: an owner
// who exported on the 3rd and took payments on the 4th must be able to export
// again.
func TestTheDedupKeyChangesWithTheArgentinaDay(t *testing.T) {
	id := uuid.New()
	day1 := time.Date(2026, 9, 11, 12, 0, 0, 0, timezone.Argentina)

	same := exportDedupKey(id, 9, 2026, day1.Add(6*time.Hour))
	if exportDedupKey(id, 9, 2026, day1) != same {
		t.Error("two requests on the same Argentina day produced different keys")
	}
	if exportDedupKey(id, 9, 2026, day1.AddDate(0, 0, 1)) == same {
		t.Error("the next day reuses the key, so the month can never be exported again")
	}
	if exportDedupKey(uuid.New(), 9, 2026, day1) == same {
		t.Error("two complexes share one key; one venue's export would deduplicate another's away")
	}
	if exportDedupKey(id, 8, 2026, day1) == same {
		t.Error("two periods share one key")
	}
}
