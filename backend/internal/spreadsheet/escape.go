// Package spreadsheet holds the defence against formula injection, shared by
// every path that hands a string to something that will read it as a
// spreadsheet cell.
//
// It is its own package because there are two such paths and they have nothing
// else in common: the payments Excel export (internal/reporting), which builds
// a file the owner downloads and opens, and the abandoned-lead webhook
// (internal/leads), which posts to the owner's Google Apps Script and lands in
// a Sheets cell. The second one was missed when the first was fixed, so the
// escape now lives where neither package owns it and a third sink can find it.
package spreadsheet

import "strings"

// formulaTriggerChars are the leading characters a spreadsheet application
// reads as "this cell opens a formula" rather than as literal text: '=' for an
// ordinary formula, and '+', '-', '@' because Excel, Lotus-1-2-3-compatible
// parsers, Google Sheets and some CSV importers still honour them as formula
// starters too. See OWASP's CSV/Excel-injection guidance.
const formulaTriggerChars = "=+-@"

// EscapeFormulaCell prefixes s with a single apostrophe when it starts with a
// formulaTriggerChars character, which every spreadsheet application reads as
// "this is text, not a formula" and does not display.
//
// H-06: client_first_name on the PUBLIC booking form (POST /api/v1/book, no
// account required) reached the payments export unescaped. RPT-10 booked
// through that form with the first name "=cmd|' /C calc'!A0", downloaded
// GET .../reports/export, and the stored cell came back as exactly that
// string — parsed as a formula by whatever spreadsheet the venue owner opens
// the file in on their own machine. The attacker never touches the venue's
// data directly; they book a court, and the payload rides in a file an
// authenticated owner trusts and opens without a second thought.
//
// H-24 is the same finding at the other sink, and it is the reason this is a
// package rather than a private helper. The abandoned-lead endpoint forwards a
// caller-supplied email to the owner's Apps Script, which writes it into a
// cell; "=1+1@example.com" is a legal address — '=' and '+' are both atext —
// so it was accepted with 202 and forwarded verbatim.
//
// Escaping here rather than validating at the input is deliberate, and H-24 is
// what makes the reason concrete: "+qa@example.com" and "-qa@example.com" are
// ordinary addresses real people hold, and both begin with a trigger
// character. Refusing them would break legitimate signups to defend a
// spreadsheet. The address is fine; the cell is the problem, so the cell is
// where the fix belongs.
//
// Apply it to every string cell a sink writes, not only the fields that carry
// free text today: a column that is safe now (a court name, a status label) is
// one schema change away from carrying it, and the guard costs nothing to keep
// on unconditionally.
func EscapeFormulaCell(s string) string {
	if s == "" {
		return s
	}
	if strings.ContainsRune(formulaTriggerChars, rune(s[0])) {
		return "'" + s
	}
	return s
}
