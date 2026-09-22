// apiServer implements gen.ServerInterface, generated from
// internal/openapi/openapi.yaml by oapi-codegen (see internal/openapi/gen).
// Every method below does exactly what the operation's entry in its domain
// module's Routes() method did before this file replaced it as the
// production registration path: call the existing handler, unchanged. Most
// domains' Routes() method is gone now that nothing calls it; audit.Handler
// and openapi.Handler keep theirs because their own package tests still
// exercise it directly. The generated path/query parameters are intentionally
// unused —
// the handlers already read them, through httpx.ReadUUIDParam, r.PathValue
// and the query-string helpers — because using them here would mean
// duplicating that parsing for no behavioural change.
//
// The guard chain (auth, ownership, role, idempotency) that used to be built
// inline in each Routes() method is NOT applied here. It is applied one
// level out, in muxAdapter.HandleFunc (routes.go), around the whole
// generated per-route dispatch function — parameter binding included —
// because oapi-codegen's std-http-server parses required parameters before
// calling into ServerInterface, and an unauthenticated or cross-tenant
// caller must still be rejected before any parameter parsing runs, exactly
// as it was when the domain handler did both jobs.
package main

import (
	"net/http"

	gen "github.com/stodulski/vibe-server/internal/openapi/gen"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

// apiServer is the ServerInterface implementation; app is the whole
// composition root because handlers span every domain.
type apiServer struct {
	app *application
}

func newAPIServer(app *application) *apiServer { return &apiServer{app: app} }

// var _ gen.ServerInterface = (*apiServer)(nil) is the compile-time half of
// the design's proof: a paths entry the document adds with no method here
// fails the build, and every method below must therefore stay one-to-one
// with the document's operations.
var _ gen.ServerInterface = (*apiServer)(nil)

// PublicsiteSitemapMoved implements gen.ServerInterface for publicsiteSitemapMoved
// (GET /api/sitemap.xml). Guarded by routeGuards; see the type comment above.
func (s *apiServer) PublicsiteSitemapMoved(w http.ResponseWriter, r *http.Request) {
	s.app.publicsite.SitemapMoved(w, r)
}

// AdminListAuditLog implements gen.ServerInterface for adminListAuditLog
// (GET /api/v1/admin/audit-log). Guarded by routeGuards; see the type comment above.
func (s *apiServer) AdminListAuditLog(w http.ResponseWriter, r *http.Request, params gen.AdminListAuditLogParams) {
	s.app.admin.ListAuditLogs(w, r)
}

// AdminListComplexes implements gen.ServerInterface for adminListComplexes
// (GET /api/v1/admin/complexes). Guarded by routeGuards; see the type comment above.
func (s *apiServer) AdminListComplexes(w http.ResponseWriter, r *http.Request, params gen.AdminListComplexesParams) {
	s.app.admin.ListComplexes(w, r)
}

// AdminGetComplex implements gen.ServerInterface for adminGetComplex
// (GET /api/v1/admin/complexes/{id}). Guarded by routeGuards; see the type comment above.
func (s *apiServer) AdminGetComplex(w http.ResponseWriter, r *http.Request, id gen.PathID) {
	s.app.admin.GetComplex(w, r)
}

// AdminGetHealthDetailed implements gen.ServerInterface for adminGetHealthDetailed
// (GET /api/v1/admin/healthcheck). Guarded by routeGuards; see the type comment above.
func (s *apiServer) AdminGetHealthDetailed(w http.ResponseWriter, r *http.Request) {
	s.app.health.Detailed(w, r)
}

// AdminGetStats implements gen.ServerInterface for adminGetStats
// (GET /api/v1/admin/stats). Guarded by routeGuards; see the type comment above.
func (s *apiServer) AdminGetStats(w http.ResponseWriter, r *http.Request) {
	s.app.admin.Stats(w, r)
}

// AdminListUsers implements gen.ServerInterface for adminListUsers
// (GET /api/v1/admin/users). Guarded by routeGuards; see the type comment above.
func (s *apiServer) AdminListUsers(w http.ResponseWriter, r *http.Request, params gen.AdminListUsersParams) {
	s.app.admin.ListUsers(w, r)
}

// AdminGetUser implements gen.ServerInterface for adminGetUser
// (GET /api/v1/admin/users/{id}). Guarded by routeGuards; see the type comment above.
func (s *apiServer) AdminGetUser(w http.ResponseWriter, r *http.Request, id gen.PathID) {
	s.app.admin.GetUser(w, r)
}

// AdminToggleUserActive implements gen.ServerInterface for adminToggleUserActive
// (PATCH /api/v1/admin/users/{id}/toggle-active). Guarded by routeGuards; see the type comment above.
func (s *apiServer) AdminToggleUserActive(w http.ResponseWriter, r *http.Request, id gen.PathID) {
	s.app.admin.ToggleUserActive(w, r)
}

// AuthForgotPassword implements gen.ServerInterface for authForgotPassword
// (POST /api/v1/auth/forgot-password). Guarded by routeGuards; see the type comment above.
func (s *apiServer) AuthForgotPassword(w http.ResponseWriter, r *http.Request) {
	s.app.auth.ForgotPassword(w, r)
}

// AuthGoogle implements gen.ServerInterface for authGoogle
// (POST /api/v1/auth/google). Guarded by routeGuards; see the type comment above.
func (s *apiServer) AuthGoogle(w http.ResponseWriter, r *http.Request) {
	s.app.auth.GoogleSignIn(w, r)
}

// AuthGoogleComplete implements gen.ServerInterface for authGoogleComplete
// (POST /api/v1/auth/google/complete). Guarded by routeGuards; see the type comment above.
func (s *apiServer) AuthGoogleComplete(w http.ResponseWriter, r *http.Request) {
	s.app.auth.GoogleComplete(w, r)
}

// AuthGoogleExchange implements gen.ServerInterface for authGoogleExchange
// (POST /api/v1/auth/google/exchange). Guarded by routeGuards; see the type comment above.
func (s *apiServer) AuthGoogleExchange(w http.ResponseWriter, r *http.Request) {
	s.app.auth.GoogleExchange(w, r)
}

// AuthGoogleRedirect implements gen.ServerInterface for authGoogleRedirect
// (POST /api/v1/auth/google/redirect). Guarded by routeGuards; see the type comment above.
func (s *apiServer) AuthGoogleRedirect(w http.ResponseWriter, r *http.Request) {
	s.app.auth.GoogleRedirect(w, r)
}

// AuthLogin implements gen.ServerInterface for authLogin
// (POST /api/v1/auth/login). Guarded by routeGuards; see the type comment above.
func (s *apiServer) AuthLogin(w http.ResponseWriter, r *http.Request) {
	s.app.auth.Login(w, r)
}

// AuthLogout implements gen.ServerInterface for authLogout
// (POST /api/v1/auth/logout). Guarded by routeGuards; see the type comment above.
func (s *apiServer) AuthLogout(w http.ResponseWriter, r *http.Request) {
	s.app.auth.Logout(w, r)
}

// AuthConfirmEmailChange implements gen.ServerInterface for authConfirmEmailChange
// (POST /api/v1/auth/confirm-email-change). Guarded by routeGuards; see the type comment above.
func (s *apiServer) AuthConfirmEmailChange(w http.ResponseWriter, r *http.Request) {
	s.app.auth.ConfirmEmailChange(w, r)
}

// AuthDeleteAccount implements gen.ServerInterface for authDeleteAccount
// (DELETE /api/v1/auth/me). Guarded by routeGuards; see the type comment above.
func (s *apiServer) AuthDeleteAccount(w http.ResponseWriter, r *http.Request) {
	s.app.auth.DeleteAccount(w, r)
}

// AuthGetCurrentUser implements gen.ServerInterface for authGetCurrentUser
// (GET /api/v1/auth/me). Guarded by routeGuards; see the type comment above.
func (s *apiServer) AuthGetCurrentUser(w http.ResponseWriter, r *http.Request) {
	s.app.auth.CurrentUser(w, r)
}

// AuthUpdateCurrentUser implements gen.ServerInterface for authUpdateCurrentUser
// (PUT /api/v1/auth/me). Guarded by routeGuards; see the type comment above.
func (s *apiServer) AuthUpdateCurrentUser(w http.ResponseWriter, r *http.Request) {
	s.app.auth.UpdateCurrentUser(w, r)
}

// AuthRefresh implements gen.ServerInterface for authRefresh
// (POST /api/v1/auth/refresh). Guarded by routeGuards; see the type comment above.
func (s *apiServer) AuthRefresh(w http.ResponseWriter, r *http.Request) {
	s.app.auth.Refresh(w, r)
}

// AuthRegister implements gen.ServerInterface for authRegister
// (POST /api/v1/auth/register). Guarded by routeGuards; see the type comment above.
func (s *apiServer) AuthRegister(w http.ResponseWriter, r *http.Request) {
	s.app.auth.Register(w, r)
}

// AuthResendVerification implements gen.ServerInterface for authResendVerification
// (POST /api/v1/auth/resend-verification). Guarded by routeGuards; see the type comment above.
func (s *apiServer) AuthResendVerification(w http.ResponseWriter, r *http.Request) {
	s.app.auth.ResendVerification(w, r)
}

// AuthResetPassword implements gen.ServerInterface for authResetPassword
// (POST /api/v1/auth/reset-password). Guarded by routeGuards; see the type comment above.
func (s *apiServer) AuthResetPassword(w http.ResponseWriter, r *http.Request) {
	s.app.auth.ResetPassword(w, r)
}

// AuthVerifyEmail implements gen.ServerInterface for authVerifyEmail
// (POST /api/v1/auth/verify-email). Guarded by routeGuards; see the type comment above.
func (s *apiServer) AuthVerifyEmail(w http.ResponseWriter, r *http.Request) {
	s.app.auth.VerifyEmail(w, r)
}

// BookingsPublicCreate implements gen.ServerInterface for bookingsPublicCreate
// (POST /api/v1/book). Guarded by routeGuards; see the type comment above.
func (s *apiServer) BookingsPublicCreate(w http.ResponseWriter, r *http.Request, params gen.BookingsPublicCreateParams) {
	s.app.bookings.PublicBook(w, r)
}

// BookingsPublicCancel implements gen.ServerInterface for bookingsPublicCancel
// (POST /api/v1/book/cancel). Guarded by routeGuards; see the type comment above.
func (s *apiServer) BookingsPublicCancel(w http.ResponseWriter, r *http.Request) {
	s.app.bookings.PublicCancel(w, r)
}

// BookingsPublicCancelInfo implements gen.ServerInterface for bookingsPublicCancelInfo
// (GET /api/v1/book/cancel-info). Guarded by routeGuards; see the type comment above.
func (s *apiServer) BookingsPublicCancelInfo(w http.ResponseWriter, r *http.Request, params gen.BookingsPublicCancelInfoParams) {
	s.app.bookings.PublicCancelInfo(w, r)
}

// BookingsPublicStatus implements gen.ServerInterface for bookingsPublicStatus
// (GET /api/v1/book/status). Guarded by routeGuards; see the type comment above.
func (s *apiServer) BookingsPublicStatus(w http.ResponseWriter, r *http.Request, params gen.BookingsPublicStatusParams) {
	s.app.bookings.PublicStatus(w, r)
}

// ComplexesList implements gen.ServerInterface for complexesList
// (GET /api/v1/complexes). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ComplexesList(w http.ResponseWriter, r *http.Request) {
	s.app.complexes.List(w, r)
}

// ComplexesCreate implements gen.ServerInterface for complexesCreate
// (POST /api/v1/complexes). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ComplexesCreate(w http.ResponseWriter, r *http.Request) {
	s.app.complexes.Create(w, r)
}

// ComplexesDelete implements gen.ServerInterface for complexesDelete
// (DELETE /api/v1/complexes/{id}). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ComplexesDelete(w http.ResponseWriter, r *http.Request, id gen.PathID) {
	s.app.complexes.Delete(w, r)
}

// ComplexesGet implements gen.ServerInterface for complexesGet
// (GET /api/v1/complexes/{id}). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ComplexesGet(w http.ResponseWriter, r *http.Request, id gen.PathID) {
	s.app.complexes.Get(w, r)
}

// ComplexesUpdate implements gen.ServerInterface for complexesUpdate
// (PUT /api/v1/complexes/{id}). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ComplexesUpdate(w http.ResponseWriter, r *http.Request, id gen.PathID, params gen.ComplexesUpdateParams) {
	s.app.complexes.Update(w, r)
}

// AuditListComplexLog implements gen.ServerInterface for auditListComplexLog
// (GET /api/v1/complexes/{id}/audit-log). Guarded by routeGuards; see the type comment above.
func (s *apiServer) AuditListComplexLog(w http.ResponseWriter, r *http.Request, id gen.PathID, params gen.AuditListComplexLogParams) {
	s.app.auditTrail.List(w, r)
}

// CourtsListBlockedSlots implements gen.ServerInterface for courtsListBlockedSlots
// (GET /api/v1/complexes/{id}/blocked-slots). Guarded by routeGuards; see the type comment above.
func (s *apiServer) CourtsListBlockedSlots(w http.ResponseWriter, r *http.Request, id gen.PathID, params gen.CourtsListBlockedSlotsParams) {
	s.app.courts.ListBlockedSlots(w, r)
}

// CourtsDeleteBlockedSlot implements gen.ServerInterface for courtsDeleteBlockedSlot
// (DELETE /api/v1/complexes/{id}/blocked-slots/{slotID}). Guarded by routeGuards; see the type comment above.
func (s *apiServer) CourtsDeleteBlockedSlot(w http.ResponseWriter, r *http.Request, id gen.PathID, slotID openapi_types.UUID) {
	s.app.courts.DeleteBlockedSlot(w, r)
}

// BookingsList implements gen.ServerInterface for bookingsList
// (GET /api/v1/complexes/{id}/bookings). Guarded by routeGuards; see the type comment above.
func (s *apiServer) BookingsList(w http.ResponseWriter, r *http.Request, id gen.PathID, params gen.BookingsListParams) {
	s.app.bookings.List(w, r)
}

// BookingsCreate implements gen.ServerInterface for bookingsCreate
// (POST /api/v1/complexes/{id}/bookings). Guarded by routeGuards; see the type comment above.
func (s *apiServer) BookingsCreate(w http.ResponseWriter, r *http.Request, id gen.PathID, params gen.BookingsCreateParams) {
	s.app.bookings.Create(w, r)
}

// BookingsGet implements gen.ServerInterface for bookingsGet
// (GET /api/v1/complexes/{id}/bookings/{bookingID}). Guarded by routeGuards; see the type comment above.
func (s *apiServer) BookingsGet(w http.ResponseWriter, r *http.Request, id gen.PathID, bookingID gen.BookingID) {
	s.app.bookings.Get(w, r)
}

// BookingsUpdate implements gen.ServerInterface for bookingsUpdate
// (PUT /api/v1/complexes/{id}/bookings/{bookingID}). Guarded by routeGuards; see the type comment above.
func (s *apiServer) BookingsUpdate(w http.ResponseWriter, r *http.Request, id gen.PathID, bookingID gen.BookingID) {
	s.app.bookings.Update(w, r)
}

// BookingsCancel implements gen.ServerInterface for bookingsCancel
// (POST /api/v1/complexes/{id}/bookings/{bookingID}/cancel). Guarded by routeGuards; see the type comment above.
func (s *apiServer) BookingsCancel(w http.ResponseWriter, r *http.Request, id gen.PathID, bookingID gen.BookingID) {
	s.app.bookings.Cancel(w, r)
}

// BookingsConfirmPayment implements gen.ServerInterface for bookingsConfirmPayment
// (POST /api/v1/complexes/{id}/bookings/{bookingID}/confirm-payment). Guarded by routeGuards; see the type comment above.
func (s *apiServer) BookingsConfirmPayment(w http.ResponseWriter, r *http.Request, id gen.PathID, bookingID gen.BookingID, params gen.BookingsConfirmPaymentParams) {
	s.app.bookings.ConfirmPayment(w, r)
}

// BookingsManualRefund implements gen.ServerInterface for bookingsManualRefund
// (POST /api/v1/complexes/{id}/bookings/{bookingID}/manual-refund). Guarded by routeGuards; see the type comment above.
func (s *apiServer) BookingsManualRefund(w http.ResponseWriter, r *http.Request, id gen.PathID, bookingID gen.BookingID, params gen.BookingsManualRefundParams) {
	s.app.bookings.ManualRefund(w, r)
}

// CashSessionsList implements gen.ServerInterface for cashSessionsList
// (GET /api/v1/complexes/{id}/cash-sessions). Guarded by routeGuards; see the type comment above.
func (s *apiServer) CashSessionsList(w http.ResponseWriter, r *http.Request, id gen.PathID, params gen.CashSessionsListParams) {
	s.app.cashbox.List(w, r)
}

// CashSessionsOpen implements gen.ServerInterface for cashSessionsOpen
// (POST /api/v1/complexes/{id}/cash-sessions). Guarded by routeGuards; see the type comment above.
func (s *apiServer) CashSessionsOpen(w http.ResponseWriter, r *http.Request, id gen.PathID, params gen.CashSessionsOpenParams) {
	s.app.cashbox.Open(w, r)
}

// CashSessionsCurrent implements gen.ServerInterface for cashSessionsCurrent
// (GET /api/v1/complexes/{id}/cash-session). Guarded by routeGuards; see the type comment above.
func (s *apiServer) CashSessionsCurrent(w http.ResponseWriter, r *http.Request, id gen.PathID) {
	s.app.cashbox.Current(w, r)
}

// CashSessionsGet implements gen.ServerInterface for cashSessionsGet
// (GET /api/v1/complexes/{id}/cash-sessions/{sessionID}). Guarded by routeGuards; see the type comment above.
func (s *apiServer) CashSessionsGet(w http.ResponseWriter, r *http.Request, id gen.PathID, sessionID gen.SessionID) {
	s.app.cashbox.Get(w, r)
}

// CashSessionsClose implements gen.ServerInterface for cashSessionsClose
// (POST /api/v1/complexes/{id}/cash-sessions/{sessionID}/close). Guarded by routeGuards; see the type comment above.
func (s *apiServer) CashSessionsClose(w http.ResponseWriter, r *http.Request, id gen.PathID, sessionID gen.SessionID, params gen.CashSessionsCloseParams) {
	s.app.cashbox.Close(w, r)
}

// CashMovementsCreate implements gen.ServerInterface for cashMovementsCreate
// (POST /api/v1/complexes/{id}/cash-sessions/{sessionID}/movements). Guarded by routeGuards; see the type comment above.
func (s *apiServer) CashMovementsCreate(w http.ResponseWriter, r *http.Request, id gen.PathID, sessionID gen.SessionID, params gen.CashMovementsCreateParams) {
	s.app.cashbox.CreateMovement(w, r)
}

// CashMovementsVoid implements gen.ServerInterface for cashMovementsVoid
// (POST /api/v1/complexes/{id}/cash-sessions/{sessionID}/movements/{movementID}/void). Guarded by routeGuards; see the type comment above.
func (s *apiServer) CashMovementsVoid(w http.ResponseWriter, r *http.Request, id gen.PathID, sessionID gen.SessionID, movementID gen.MovementID, params gen.CashMovementsVoidParams) {
	s.app.cashbox.VoidMovement(w, r)
}

// ClientsList implements gen.ServerInterface for clientsList
// (GET /api/v1/complexes/{id}/clients). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ClientsList(w http.ResponseWriter, r *http.Request, id gen.PathID, params gen.ClientsListParams) {
	s.app.clients.List(w, r)
}

// ClientsGet implements gen.ServerInterface for clientsGet
// (GET /api/v1/complexes/{id}/clients/{clientID}). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ClientsGet(w http.ResponseWriter, r *http.Request, id gen.PathID, clientID gen.ClientID) {
	s.app.clients.Get(w, r)
}

// ClientsUpdate implements gen.ServerInterface for clientsUpdate
// (PUT /api/v1/complexes/{id}/clients/{clientID}). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ClientsUpdate(w http.ResponseWriter, r *http.Request, id gen.PathID, clientID gen.ClientID) {
	s.app.clients.Update(w, r)
}

// CourtsList implements gen.ServerInterface for courtsList
// (GET /api/v1/complexes/{id}/courts). Guarded by routeGuards; see the type comment above.
func (s *apiServer) CourtsList(w http.ResponseWriter, r *http.Request, id gen.PathID) {
	s.app.courts.List(w, r)
}

// CourtsCreate implements gen.ServerInterface for courtsCreate
// (POST /api/v1/complexes/{id}/courts). Guarded by routeGuards; see the type comment above.
func (s *apiServer) CourtsCreate(w http.ResponseWriter, r *http.Request, id gen.PathID) {
	s.app.courts.Create(w, r)
}

// CourtsDelete implements gen.ServerInterface for courtsDelete
// (DELETE /api/v1/complexes/{id}/courts/{courtID}). Guarded by routeGuards; see the type comment above.
func (s *apiServer) CourtsDelete(w http.ResponseWriter, r *http.Request, id gen.PathID, courtID gen.CourtID) {
	s.app.courts.Delete(w, r)
}

// CourtsUpdate implements gen.ServerInterface for courtsUpdate
// (PUT /api/v1/complexes/{id}/courts/{courtID}). Guarded by routeGuards; see the type comment above.
func (s *apiServer) CourtsUpdate(w http.ResponseWriter, r *http.Request, id gen.PathID, courtID gen.CourtID, params gen.CourtsUpdateParams) {
	s.app.courts.Update(w, r)
}

// CourtsBlockSlot implements gen.ServerInterface for courtsBlockSlot
// (POST /api/v1/complexes/{id}/courts/{courtID}/block). Guarded by routeGuards; see the type comment above.
func (s *apiServer) CourtsBlockSlot(w http.ResponseWriter, r *http.Request, id gen.PathID, courtID gen.CourtID) {
	s.app.courts.BlockSlot(w, r)
}

// CourtsUpdatePrices implements gen.ServerInterface for courtsUpdatePrices
// (PUT /api/v1/complexes/{id}/courts/{courtID}/prices). Guarded by routeGuards; see the type comment above.
func (s *apiServer) CourtsUpdatePrices(w http.ResponseWriter, r *http.Request, id gen.PathID, courtID gen.CourtID, params gen.CourtsUpdatePricesParams) {
	s.app.courts.UpdatePrices(w, r)
}

// RealtimeStream implements gen.ServerInterface for realtimeStream
// (GET /api/v1/complexes/{id}/events). Guarded by routeGuards; see the type comment above.
func (s *apiServer) RealtimeStream(w http.ResponseWriter, r *http.Request, id gen.PathID) {
	s.app.realtime.Stream(w, r)
}

// ComplexesDisconnectMercadoPago implements gen.ServerInterface for complexesDisconnectMercadoPago
// (DELETE /api/v1/complexes/{id}/mp/connect). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ComplexesDisconnectMercadoPago(w http.ResponseWriter, r *http.Request, id gen.PathID) {
	s.app.complexes.DisconnectMercadoPago(w, r)
}

// ComplexesConnectMercadoPago implements gen.ServerInterface for complexesConnectMercadoPago
// (POST /api/v1/complexes/{id}/mp/connect). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ComplexesConnectMercadoPago(w http.ResponseWriter, r *http.Request, id gen.PathID) {
	s.app.complexes.ConnectMercadoPago(w, r)
}

// ComplexesMercadoPagoStatus implements gen.ServerInterface for complexesMercadoPagoStatus
// (GET /api/v1/complexes/{id}/mp/status). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ComplexesMercadoPagoStatus(w http.ResponseWriter, r *http.Request, id gen.PathID) {
	s.app.complexes.MercadoPagoStatus(w, r)
}

// ReportingExportPaymentsExcel implements gen.ServerInterface for reportingExportPaymentsExcel
// (GET /api/v1/complexes/{id}/reports/export). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ReportingExportPaymentsExcel(w http.ResponseWriter, r *http.Request, id gen.PathID, params gen.ReportingExportPaymentsExcelParams) {
	// The handler is deprecated in favour of the export job, and the route
	// still has to be registered until the frontend has switched — the whole
	// point of deprecating rather than deleting it. The marker is what tells
	// every OTHER caller to stop; this one is the route itself.
	s.app.reporting.ExportPaymentsExcel(w, r) //nolint:staticcheck // SA1019: see above
}

// ReportingCreatePaymentsExport implements gen.ServerInterface for reportingCreatePaymentsExport
// (POST /api/v1/complexes/{id}/reports/exports). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ReportingCreatePaymentsExport(w http.ResponseWriter, r *http.Request, id gen.PathID) {
	s.app.reporting.CreatePaymentsExport(w, r)
}

// ReportingGetPaymentsExport implements gen.ServerInterface for reportingGetPaymentsExport
// (GET /api/v1/complexes/{id}/reports/exports/{exportID}). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ReportingGetPaymentsExport(w http.ResponseWriter, r *http.Request, id gen.PathID, exportID gen.ExportID) {
	s.app.reporting.GetPaymentsExport(w, r)
}

// ReportingGetMonthlyReport implements gen.ServerInterface for reportingGetMonthlyReport
// (GET /api/v1/complexes/{id}/reports/monthly). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ReportingGetMonthlyReport(w http.ResponseWriter, r *http.Request, id gen.PathID, params gen.ReportingGetMonthlyReportParams) {
	s.app.reporting.GetMonthlyReport(w, r)
}

// ComplexesUpdateSchedules implements gen.ServerInterface for complexesUpdateSchedules
// (PUT /api/v1/complexes/{id}/schedules). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ComplexesUpdateSchedules(w http.ResponseWriter, r *http.Request, id gen.PathID) {
	s.app.complexes.UpdateSchedules(w, r)
}

// ReportingGetDashboardStats implements gen.ServerInterface for reportingGetDashboardStats
// (GET /api/v1/complexes/{id}/stats). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ReportingGetDashboardStats(w http.ResponseWriter, r *http.Request, id gen.PathID) {
	s.app.reporting.GetDashboardStats(w, r)
}

// ReportingGetClientInsights implements gen.ServerInterface for reportingGetClientInsights
// (GET /api/v1/complexes/{id}/stats/clients). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ReportingGetClientInsights(w http.ResponseWriter, r *http.Request, id gen.PathID) {
	s.app.reporting.GetClientInsights(w, r)
}

// ReportingGetOccupancyChart implements gen.ServerInterface for reportingGetOccupancyChart
// (GET /api/v1/complexes/{id}/stats/occupancy). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ReportingGetOccupancyChart(w http.ResponseWriter, r *http.Request, id gen.PathID, params gen.ReportingGetOccupancyChartParams) {
	s.app.reporting.GetOccupancyChart(w, r)
}

// ReportingGetRevenueChart implements gen.ServerInterface for reportingGetRevenueChart
// (GET /api/v1/complexes/{id}/stats/revenue). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ReportingGetRevenueChart(w http.ResponseWriter, r *http.Request, id gen.PathID, params gen.ReportingGetRevenueChartParams) {
	s.app.reporting.GetRevenueChart(w, r)
}

// ComplexesDeleteUpload implements gen.ServerInterface for complexesDeleteUpload
// (DELETE /api/v1/complexes/{id}/uploads). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ComplexesDeleteUpload(w http.ResponseWriter, r *http.Request, id gen.PathID) {
	s.app.complexes.DeleteUpload(w, r)
}

// ComplexesPresignUpload implements gen.ServerInterface for complexesPresignUpload
// (POST /api/v1/complexes/{id}/uploads/presign). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ComplexesPresignUpload(w http.ResponseWriter, r *http.Request, id gen.PathID) {
	s.app.complexes.PresignUpload(w, r)
}

// OpenapiGetDocs implements gen.ServerInterface for openapiGetDocs
// (GET /api/v1/docs). Guarded by routeGuards; see the type comment above.
func (s *apiServer) OpenapiGetDocs(w http.ResponseWriter, r *http.Request) {
	s.app.openapi.Docs(w, r)
}

// OpenapiGetCatalog implements gen.ServerInterface for openapiGetCatalog
// (GET /.well-known/api-catalog). Guarded by routeGuards; see the type comment above.
func (s *apiServer) OpenapiGetCatalog(w http.ResponseWriter, r *http.Request) {
	s.app.openapi.Catalog(w, r)
}

// HealthCheck implements gen.ServerInterface for healthCheck
// (GET /api/v1/healthcheck). Guarded by routeGuards; see the type comment above.
func (s *apiServer) HealthCheck(w http.ResponseWriter, r *http.Request) {
	s.app.health.Check(w, r)
}

// LivenessCheck implements gen.ServerInterface for livenessCheck
// (GET /api/v1/livez). Guarded by routeGuards; see the type comment above.
func (s *apiServer) LivenessCheck(w http.ResponseWriter, r *http.Request) {
	s.app.health.Live(w, r)
}

// OpenapiGetJSON implements gen.ServerInterface for openapiGetJSON
// (GET /api/v1/openapi.json). Guarded by routeGuards; see the type comment above.
func (s *apiServer) OpenapiGetJSON(w http.ResponseWriter, r *http.Request) {
	s.app.openapi.JSON(w, r)
}

// OpenapiGetYAML implements gen.ServerInterface for openapiGetYAML
// (GET /api/v1/openapi.yaml). Guarded by routeGuards; see the type comment above.
func (s *apiServer) OpenapiGetYAML(w http.ResponseWriter, r *http.Request) {
	s.app.openapi.YAML(w, r)
}

// PlacesAutocomplete implements gen.ServerInterface for placesAutocomplete
// (GET /api/v1/places/autocomplete). Guarded by routeGuards; see the type comment above.
func (s *apiServer) PlacesAutocomplete(w http.ResponseWriter, r *http.Request, params gen.PlacesAutocompleteParams) {
	s.app.places.Autocomplete(w, r)
}

// PlacesDetails implements gen.ServerInterface for placesDetails
// (GET /api/v1/places/details). Guarded by routeGuards; see the type comment above.
func (s *apiServer) PlacesDetails(w http.ResponseWriter, r *http.Request, params gen.PlacesDetailsParams) {
	s.app.places.Details(w, r)
}

// ProductsList implements gen.ServerInterface for productsList
// (GET /api/v1/complexes/{id}/products). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ProductsList(w http.ResponseWriter, r *http.Request, id gen.PathID, params gen.ProductsListParams) {
	s.app.products.List(w, r)
}

// ProductsCreate implements gen.ServerInterface for productsCreate
// (POST /api/v1/complexes/{id}/products). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ProductsCreate(w http.ResponseWriter, r *http.Request, id gen.PathID, params gen.ProductsCreateParams) {
	s.app.products.Create(w, r)
}

// ProductsGet implements gen.ServerInterface for productsGet
// (GET /api/v1/complexes/{id}/products/{productID}). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ProductsGet(w http.ResponseWriter, r *http.Request, id gen.PathID, productID gen.ProductID) {
	s.app.products.Get(w, r)
}

// ProductsUpdate implements gen.ServerInterface for productsUpdate
// (PATCH /api/v1/complexes/{id}/products/{productID}). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ProductsUpdate(w http.ResponseWriter, r *http.Request, id gen.PathID, productID gen.ProductID, params gen.ProductsUpdateParams) {
	s.app.products.Update(w, r)
}

// ProductsRestock implements gen.ServerInterface for productsRestock
// (POST /api/v1/complexes/{id}/products/{productID}/restock). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ProductsRestock(w http.ResponseWriter, r *http.Request, id gen.PathID, productID gen.ProductID, params gen.ProductsRestockParams) {
	s.app.products.Restock(w, r)
}

// ProductsAdjust implements gen.ServerInterface for productsAdjust
// (POST /api/v1/complexes/{id}/products/{productID}/adjustments). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ProductsAdjust(w http.ResponseWriter, r *http.Request, id gen.PathID, productID gen.ProductID, params gen.ProductsAdjustParams) {
	s.app.products.Adjust(w, r)
}

// ProductsListStockMovements implements gen.ServerInterface for productsListStockMovements
// (GET /api/v1/complexes/{id}/products/{productID}/stock-movements). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ProductsListStockMovements(w http.ResponseWriter, r *http.Request, id gen.PathID, productID gen.ProductID, params gen.ProductsListStockMovementsParams) {
	s.app.products.ListStockMovements(w, r)
}

// ComplexesGetPublic implements gen.ServerInterface for complexesGetPublic
// (GET /api/v1/public/complexes/{slug}). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ComplexesGetPublic(w http.ResponseWriter, r *http.Request, slug gen.PathSlug) {
	s.app.complexes.GetPublic(w, r)
}

// CourtsPublicAvailability implements gen.ServerInterface for courtsPublicAvailability
// (GET /api/v1/public/complexes/{slug}/availability). Guarded by routeGuards; see the type comment above.
func (s *apiServer) CourtsPublicAvailability(w http.ResponseWriter, r *http.Request, slug gen.PathSlug, params gen.CourtsPublicAvailabilityParams) {
	s.app.courts.Availability(w, r)
}

// LeadsCaptureAbandonedRegistration implements gen.ServerInterface for leadsCaptureAbandonedRegistration
// (POST /api/v1/public/leads/abandoned-registration). Guarded by routeGuards; see the type comment above.
func (s *apiServer) LeadsCaptureAbandonedRegistration(w http.ResponseWriter, r *http.Request) {
	s.app.leads.CaptureAbandonedRegistration(w, r)
}

// PublicsitePrerender implements gen.ServerInterface for publicsitePrerender
// (GET /api/v1/public/prerender/{slug}). Guarded by routeGuards; see the type comment above.
func (s *apiServer) PublicsitePrerender(w http.ResponseWriter, r *http.Request, slug gen.PathSlug) {
	s.app.publicsite.Prerender(w, r)
}

// PublicsiteSitemap implements gen.ServerInterface for publicsiteSitemap
// (GET /api/v1/sitemap.xml). Guarded by routeGuards; see the type comment above.
func (s *apiServer) PublicsiteSitemap(w http.ResponseWriter, r *http.Request) {
	s.app.publicsite.Sitemap(w, r)
}

// ComplexesSlugAvailable implements gen.ServerInterface for complexesSlugAvailable
// (GET /api/v1/slug-available). Guarded by routeGuards; see the type comment above.
func (s *apiServer) ComplexesSlugAvailable(w http.ResponseWriter, r *http.Request, params gen.ComplexesSlugAvailableParams) {
	s.app.complexes.SlugAvailable(w, r)
}

// PaymentsMercadoPagoWebhook implements gen.ServerInterface for paymentsMercadoPagoWebhook
// (POST /api/v1/webhooks/mercadopago). Guarded by routeGuards; see the type comment above.
func (s *apiServer) PaymentsMercadoPagoWebhook(w http.ResponseWriter, r *http.Request, params gen.PaymentsMercadoPagoWebhookParams) {
	s.app.payments.MercadoPagoWebhook(w, r)
}

// BookingsWhatsAppVerifyWebhook implements gen.ServerInterface for bookingsWhatsAppVerifyWebhook
// (GET /api/v1/webhooks/whatsapp). Guarded by routeGuards; see the type comment above.
func (s *apiServer) BookingsWhatsAppVerifyWebhook(w http.ResponseWriter, r *http.Request, params gen.BookingsWhatsAppVerifyWebhookParams) {
	s.app.bookings.WhatsAppVerify(w, r)
}

// BookingsWhatsAppWebhook implements gen.ServerInterface for bookingsWhatsAppWebhook
// (POST /api/v1/webhooks/whatsapp). Guarded by routeGuards; see the type comment above.
func (s *apiServer) BookingsWhatsAppWebhook(w http.ResponseWriter, r *http.Request) {
	s.app.bookings.WhatsAppWebhook(w, r)
}
