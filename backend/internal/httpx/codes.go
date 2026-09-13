package httpx

// Error codes are the API's contract for anything a person will read.
//
// The backend returns the code; the frontend maps it to text from its own
// locale files. That keeps one place responsible for what the user reads, and
// stops the same sentence existing in two repositories that drift apart.
//
// The rule is only about copy the frontend renders. Text the backend sends
// somewhere the frontend never sees — an email, a spreadsheet, a meta tag, a
// card-statement description — is the backend's to word, and lives in the
// copy.go of whichever module produces it.
//
// A code is stable API. Renaming one breaks the frontend's mapping as surely
// as renaming a JSON field would.
const (
	// CodeSlugTaken means the requested public URL already belongs to another
	// complex. Only the server can know this: it needs the database.
	CodeSlugTaken = "slug_taken"

	// CodeDepositOver100 means the complex is configured with a deposit
	// percentage above 100.
	CodeDepositOver100 = "deposit_percentage_over_100"

	// CodeDepositExceedsPrice means the requested deposit is larger than the
	// booking it is for.
	CodeDepositExceedsPrice = "deposit_exceeds_price"

	// CodeMonthOutOfRange means the requested report month is not 1-12.
	CodeMonthOutOfRange = "month_out_of_range"

	// CodeInvalidReportPeriod means month or year on a report request was not
	// a whole number at all — as distinct from CodeMonthOutOfRange, which
	// means it parsed fine but fell outside 1-12. See H-05: without telling
	// these apart, `month=abc` silently fell back to the current month and
	// answered 200 with that month's money, while `month=13` was correctly
	// refused with a 400.
	CodeInvalidReportPeriod = "invalid_report_period"

	// CodeFuturePeriod means a report was requested for a period that has not
	// happened yet.
	CodeFuturePeriod = "report_period_in_future"

	// CodeBeforeComplexExisted means the requested period predates the
	// complex, so there is nothing to report on.
	CodeBeforeComplexExisted = "report_period_before_complex_existed"

	// CodeExportTooLarge means the requested period holds more payments than
	// the spreadsheet export will build. The owner is told rather than handed
	// a file that quietly stops partway: a ledger missing its last fortnight
	// is worse than no ledger, because it looks complete.
	CodeExportTooLarge = "report_export_too_large"

	// CodeExportTimedOut means the export did not finish inside its budget.
	CodeExportTimedOut = "report_export_timed_out"

	// CodeExportFailed means a background export job dead-lettered for a
	// reason the owner cannot act on. It is deliberately the only thing said:
	// a failed job's last_error is a Go error message, and putting that on
	// the wire hands a tenant our internals.
	CodeExportFailed = "report_export_failed"

	// CodeExportExpired means the export finished but its file has passed the
	// 24 hour retention the storage lifecycle rule enforces. The job row
	// outlives the object it points at — jobs are kept for seven days — so
	// this is the difference between "gone" and "never existed".
	CodeExportExpired = "export_expired"

	// CodePriceRequired means no price rule covers some part of the owner
	// booking's span, so the caller must supply an explicit price instead of
	// relying on the computed one.
	CodePriceRequired = "price_required"
)
