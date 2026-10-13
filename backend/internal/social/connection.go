package social

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"

	"github.com/stodulski/vibe-server/internal/jobs"
)

// TaskConnection is the queue task that mails the result of a connection check.
const TaskConnection = "social_connection_email"

// RequiredPermissions are the permissions the publisher relies on. A check
// names each one the token does not hold.
var RequiredPermissions = []string{
	"pages_show_list",
	"pages_read_engagement",
	"pages_manage_posts",
	"instagram_basic",
	"instagram_content_publish",
	"business_management",
}

// ConnectionReport is the outcome of one check against Meta. It holds no secret.
type ConnectionReport struct {
	PageName   string
	PageID     string
	IGUsername string
	IGUserID   string
	// Missing lists the required permissions the token does not hold.
	Missing []string
	// Problems lists what failed, in plain text, one per entry.
	Problems []string
}

// OK reports whether the connection can publish: no problems, no missing permission.
func (r ConnectionReport) OK() bool {
	return len(r.Problems) == 0 && len(r.Missing) == 0
}

// Lines is the report as the body of the email: plain text, no secret.
func (r ConnectionReport) Lines() []string {
	lines := []string{connPageLine(r), connInstagramLine(r), connPermissionsLine(r)}
	for _, p := range r.Problems {
		lines = append(lines, "Error: "+p+".")
	}
	return lines
}

func connPageLine(r ConnectionReport) string {
	if r.PageID == "" {
		return "Página: no encontrada."
	}
	return fmt.Sprintf("Página: %s (%s).", r.PageName, r.PageID)
}

func connInstagramLine(r ConnectionReport) string {
	if r.IGUserID == "" {
		return "Instagram: sin cuenta profesional vinculada."
	}
	return fmt.Sprintf("Instagram: @%s (%s).", r.IGUsername, r.IGUserID)
}

func connPermissionsLine(r ConnectionReport) string {
	if len(r.Missing) == 0 {
		return "Permisos: todos concedidos."
	}
	return "Permisos faltantes: " + strings.Join(r.Missing, ", ") + "."
}

// ConnectionChecker asks Meta for the state of the connection. It never fails:
// a failure is a problem in the report.
type ConnectionChecker interface {
	CheckConnection(ctx context.Context) ConnectionReport
}

// Connector checks the connection and queues one email with the result. The
// dedup key is a digest of the token and the result, so a boot that finds the
// same token in the same state sends nothing new.
type Connector struct {
	checker ConnectionChecker
	queue   Enqueuer
	to      string
	token   string
	checks  []Check
	logger  *slog.Logger
}

// ReportLine is one line of the connection email. Problem makes the verdict
// "con problemas"; an informational line never does.
type ReportLine struct {
	Text    string
	Problem bool
}

// Check adds one line to the connection email. It sees the Meta report, so a
// check can depend on the account the report found.
type Check interface {
	Check(ctx context.Context, report ConnectionReport) ReportLine
}

// NewConnector returns a Connector that mails to. A blank token disables it.
func NewConnector(checker ConnectionChecker, queue Enqueuer, to, token string, logger *slog.Logger) *Connector {
	return &Connector{checker: checker, queue: queue, to: to, token: token, logger: logger}
}

// WithChecks adds checks whose lines follow the Meta lines in the email.
func (c *Connector) WithChecks(checks ...Check) *Connector {
	c.checks = append(c.checks, checks...)
	return c
}

// Run checks the connection once and queues the report email. It returns the
// report and whether a check ran.
func (c *Connector) Run(ctx context.Context) (ConnectionReport, bool) {
	if c.token == "" {
		c.logger.Info("social: meta connection check skipped, META_SOCIAL_ACCESS_TOKEN is not set")
		return ConnectionReport{}, false
	}
	report := c.checker.CheckConnection(ctx)
	lines := report.Lines()
	ok := report.OK()
	for _, check := range c.checks {
		line := check.Check(ctx, report)
		lines = append(lines, line.Text)
		ok = ok && !line.Problem
	}

	verdict := "OK"
	if !ok {
		verdict = "con problemas"
	}
	payload := ReportPayload{To: c.to, Subject: "Conexión con Meta: " + verdict, Lines: lines}
	key := jobs.DedupKey(TaskConnection, c.to, digest(c.token, strings.Join(lines, "\n")))
	c.queue.Enqueue(TaskConnection, payload, key)

	c.logger.Info("social: meta connection checked",
		"ok", report.OK(),
		"page_id", report.PageID,
		"ig_user_id", report.IGUserID,
		"missing_permissions", len(report.Missing),
		"problems", len(report.Problems),
		"token_fingerprint", fingerprint(c.token),
	)
	return report, true
}

// digest binds the token to the result without storing either.
func digest(token, result string) string {
	sum := sha256.Sum256([]byte(token + "\x00" + result))
	return hex.EncodeToString(sum[:])
}

// fingerprint identifies a token in logs without showing it: the first eight
// hex digits of its SHA-256, enough to see that the token changed.
func fingerprint(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])[:8]
}
