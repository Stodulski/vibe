// Package main: apiServerParamError and routeGuards are the two pieces
// routes.go's muxAdapter needs to register the generated ServerInterface
// through gen.HandlerWithOptions while keeping every route's guard chain and
// error-response shape exactly as its domain module's own Routes() method
// built them.
package main

import (
	"net/http"

	"github.com/stodulski/vibe-server/internal/httpx"
)

// guardKind is the base protection a route needs, before any idempotency
// replay. It mirrors the "owner"/"superAdmin"/"protected" combinators each
// domain's Routes() method used to build locally — one shared vocabulary
// instead of fourteen near-identical copies.
type guardKind int

const (
	// guardPublic carries no guard: the route is reachable without a
	// session, exactly as it was when it was registered with no wrapping at
	// all.
	guardPublic guardKind = iota
	// guardAuth is guards.RequireAuth alone.
	guardAuth
	// guardOwner is guards.RequireAuth(guards.RequireComplexOwner(next)).
	guardOwner
	// guardSuperAdmin is guards.RequireAuth(guards.RequireSuperAdmin(next)).
	guardSuperAdmin
)

// routeGuard is one route's protection: its base guard, plus, for the
// handful of write endpoints that replay Idempotency-Key, the scope its
// Routes() method passed to guards.Idempotent. Idempotency composes with the
// base guard rather than replacing it, and — as it did before this table —
// wraps on the inside, so the guard still runs first against an
// unauthenticated or cross-tenant caller.
type routeGuard struct {
	kind          guardKind
	idempotentKey string
}

// routeGuards maps every "METHOD /path" this application registers from
// internal/openapi/openapi.yaml (the same key shape as apiSurface in
// routes_surface_test.go and middleware.CrossTenantRoutes) to the guard
// chain its domain module used to build inline in its own Routes() method.
// muxAdapter.HandleFunc applies it, wrapped around the whole generated
// per-route dispatch function, when gen.HandlerWithOptions registers that
// route — see the comment there for why the guard must wrap parameter
// binding rather than run inside apiServer.
//
// TestRouteGuardsMatchesInventory (apiserver_test.go) asserts this map's key
// set is exactly apiSurface: every registered route has an entry, and no
// entry survives a route that was renamed or removed.
var routeGuards = map[string]routeGuard{
	"GET /api/sitemap.xml":                                             {kind: guardPublic},
	"GET /api/v1/admin/audit-log":                                      {kind: guardSuperAdmin},
	"GET /api/v1/admin/complexes":                                      {kind: guardSuperAdmin},
	"GET /api/v1/admin/complexes/{id}":                                 {kind: guardSuperAdmin},
	"GET /api/v1/admin/healthcheck":                                    {kind: guardSuperAdmin},
	"GET /api/v1/admin/stats":                                          {kind: guardSuperAdmin},
	"GET /api/v1/admin/users":                                          {kind: guardSuperAdmin},
	"GET /api/v1/admin/users/{id}":                                     {kind: guardSuperAdmin},
	"PATCH /api/v1/admin/users/{id}/toggle-active":                     {kind: guardSuperAdmin},
	"POST /api/v1/auth/forgot-password":                                {kind: guardPublic},
	"POST /api/v1/auth/google":                                         {kind: guardPublic},
	"POST /api/v1/auth/google/complete":                                {kind: guardPublic},
	"POST /api/v1/auth/login":                                          {kind: guardPublic},
	"POST /api/v1/auth/logout":                                         {kind: guardPublic},
	"DELETE /api/v1/auth/me":                                           {kind: guardAuth},
	"GET /api/v1/auth/me":                                              {kind: guardAuth},
	"PUT /api/v1/auth/me":                                              {kind: guardAuth},
	"POST /api/v1/auth/refresh":                                        {kind: guardPublic},
	"POST /api/v1/auth/register":                                       {kind: guardPublic},
	"POST /api/v1/auth/resend-verification":                            {kind: guardPublic},
	"POST /api/v1/auth/reset-password":                                 {kind: guardPublic},
	"POST /api/v1/auth/verify-email":                                   {kind: guardPublic},
	"POST /api/v1/book":                                                {kind: guardPublic, idempotentKey: "public-book"},
	"POST /api/v1/book/cancel":                                         {kind: guardPublic},
	"GET /api/v1/book/cancel-info":                                     {kind: guardPublic},
	"GET /api/v1/book/status":                                          {kind: guardPublic},
	"GET /api/v1/complexes":                                            {kind: guardAuth},
	"POST /api/v1/complexes":                                           {kind: guardAuth},
	"DELETE /api/v1/complexes/{id}":                                    {kind: guardOwner},
	"GET /api/v1/complexes/{id}":                                       {kind: guardOwner},
	"PUT /api/v1/complexes/{id}":                                       {kind: guardOwner},
	"GET /api/v1/complexes/{id}/audit-log":                             {kind: guardOwner},
	"GET /api/v1/complexes/{id}/blocked-slots":                         {kind: guardOwner},
	"DELETE /api/v1/complexes/{id}/blocked-slots/{slotID}":             {kind: guardOwner},
	"GET /api/v1/complexes/{id}/bookings":                              {kind: guardOwner},
	"POST /api/v1/complexes/{id}/bookings":                             {kind: guardOwner, idempotentKey: "owner-book"},
	"GET /api/v1/complexes/{id}/bookings/{bookingID}":                  {kind: guardOwner},
	"PUT /api/v1/complexes/{id}/bookings/{bookingID}":                  {kind: guardOwner},
	"POST /api/v1/complexes/{id}/bookings/{bookingID}/cancel":          {kind: guardOwner},
	"POST /api/v1/complexes/{id}/bookings/{bookingID}/confirm-payment": {kind: guardOwner, idempotentKey: "confirm-payment"},
	"POST /api/v1/complexes/{id}/bookings/{bookingID}/manual-refund":   {kind: guardOwner, idempotentKey: "manual-refund"},
	"GET /api/v1/complexes/{id}/clients":                               {kind: guardOwner},
	"GET /api/v1/complexes/{id}/clients/{clientID}":                    {kind: guardOwner},
	"PUT /api/v1/complexes/{id}/clients/{clientID}":                    {kind: guardOwner},
	"GET /api/v1/complexes/{id}/courts":                                {kind: guardOwner},
	"POST /api/v1/complexes/{id}/courts":                               {kind: guardOwner},
	"DELETE /api/v1/complexes/{id}/courts/{courtID}":                   {kind: guardOwner},
	"PUT /api/v1/complexes/{id}/courts/{courtID}":                      {kind: guardOwner},
	"POST /api/v1/complexes/{id}/courts/{courtID}/block":               {kind: guardOwner},
	"PUT /api/v1/complexes/{id}/courts/{courtID}/prices":               {kind: guardOwner},
	"GET /api/v1/complexes/{id}/events":                                {kind: guardOwner},
	"DELETE /api/v1/complexes/{id}/mp/connect":                         {kind: guardOwner},
	"POST /api/v1/complexes/{id}/mp/connect":                           {kind: guardOwner},
	"GET /api/v1/complexes/{id}/mp/status":                             {kind: guardOwner},
	"GET /api/v1/complexes/{id}/reports/export":                        {kind: guardOwner},
	"GET /api/v1/complexes/{id}/reports/monthly":                       {kind: guardOwner},
	"PUT /api/v1/complexes/{id}/schedules":                             {kind: guardOwner},
	"GET /api/v1/complexes/{id}/stats":                                 {kind: guardOwner},
	"GET /api/v1/complexes/{id}/stats/clients":                         {kind: guardOwner},
	"GET /api/v1/complexes/{id}/stats/occupancy":                       {kind: guardOwner},
	"GET /api/v1/complexes/{id}/stats/revenue":                         {kind: guardOwner},
	"DELETE /api/v1/complexes/{id}/uploads":                            {kind: guardOwner},
	"POST /api/v1/complexes/{id}/uploads/presign":                      {kind: guardOwner},
	"GET /api/v1/docs":                                                 {kind: guardPublic},
	"GET /api/v1/healthcheck":                                          {kind: guardPublic},
	"GET /api/v1/livez":                                                {kind: guardPublic},
	"GET /api/v1/openapi.json":                                         {kind: guardPublic},
	"GET /api/v1/openapi.yaml":                                         {kind: guardPublic},
	"GET /api/v1/places/autocomplete":                                  {kind: guardAuth},
	"GET /api/v1/places/details":                                       {kind: guardAuth},
	"GET /api/v1/public/complexes/{slug}":                              {kind: guardPublic},
	"GET /api/v1/public/complexes/{slug}/availability":                 {kind: guardPublic},
	"POST /api/v1/public/leads/abandoned-registration":                 {kind: guardPublic},
	"GET /api/v1/public/prerender/{slug}":                              {kind: guardPublic},
	"GET /api/v1/sitemap.xml":                                          {kind: guardPublic},
	"GET /api/v1/slug-available":                                       {kind: guardAuth},
	"POST /api/v1/webhooks/mercadopago":                                {kind: guardPublic},
	"GET /api/v1/webhooks/whatsapp":                                    {kind: guardPublic},
	"POST /api/v1/webhooks/whatsapp":                                   {kind: guardPublic},
}

// guard applies g to next using the application's real guard implementations
// (auth, ownership, role, idempotency) — the same *middleware.Middleware
// methods httpx.Guards has always wrapped domain handlers with.
func guard(guards httpx.Guards, g routeGuard, next http.HandlerFunc) http.HandlerFunc {
	if g.idempotentKey != "" {
		next = guards.Idempotent(g.idempotentKey)(next)
	}
	switch g.kind {
	case guardPublic:
		// No guard: the route is reachable without a session.
	case guardAuth:
		next = guards.RequireAuth(next)
	case guardOwner:
		next = guards.RequireAuth(guards.RequireComplexOwner(next))
	case guardSuperAdmin:
		next = guards.RequireAuth(guards.RequireSuperAdmin(next))
	}
	return next
}

// apiServerParamError is gen.StdHTTPServerOptions.ErrorHandlerFunc. The
// generated wrapper's own path/query-parameter binding can reject a request
// (a malformed "id" that is not a UUID, a missing required query parameter)
// before any guard or handler runs. Answering through app.respond.BadRequest
// keeps the status (400) and envelope (`{"error": ...}`) identical to every
// other bad-request response in this API; only the message text can differ
// from what the handler's own parsing would have said, and no test pins
// that text.
func (app *application) apiServerParamError(w http.ResponseWriter, r *http.Request, err error) {
	app.respond.BadRequest(w, r, err)
}
