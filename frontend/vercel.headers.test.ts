// @vitest-environment node
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/**
 * The security headers are a deploy-time contract, not application code: they
 * live in `vercel.json` and nothing in the bundle can observe them. This suite
 * reads that file from disk so a header that is deleted, renamed or weakened
 * fails CI instead of silently shipping.
 *
 * These headers (SEC-03 / DEP-02) enforce immediately — none of them can
 * break a page the way a wrong CSP can, which is why they ship on their own
 * here. The Content-Security-Policy is held back on `feat/audit-csp-report-only`
 * until there is a collector to send its violation reports to: a
 * Report-Only policy nobody receives reports from is a header that costs
 * bytes and proves nothing.
 */

interface VercelHeader {
  key: string;
  value: string;
}

interface VercelHeaderRule {
  source: string;
  headers: VercelHeader[];
}

const config = JSON.parse(readFileSync(fileURLToPath(new URL('./vercel.json', import.meta.url)), 'utf8')) as {
  headers: VercelHeaderRule[];
};

/** The rule that matches every path — where the site-wide defenses belong. */
const globalRule = config.headers.find((rule) => rule.source === '/(.*)');

function headerValue(key: string): string | undefined {
  return globalRule?.headers.find((header) => header.key === key)?.value;
}

describe('vercel.json security headers', () => {
  it('applies a rule to every path', () => {
    expect(globalRule).toBeDefined();
  });

  it.each([
    ['Strict-Transport-Security', 'max-age=63072000; includeSubDomains; preload'],
    ['X-Content-Type-Options', 'nosniff'],
    ['X-Frame-Options', 'DENY'],
    ['Referrer-Policy', 'strict-origin-when-cross-origin'],
    ['Permissions-Policy', undefined],
  ])('sets %s on every path', (key, expected) => {
    const value = headerValue(key);
    expect(value).toBeTruthy();
    if (expected !== undefined) expect(value).toBe(expected);
  });

  it('denies camera, microphone and payment in Permissions-Policy', () => {
    const policy = headerValue('Permissions-Policy') ?? '';
    expect(policy).toContain('camera=()');
    expect(policy).toContain('microphone=()');
    expect(policy).toContain('payment=()');
  });

  // Until the CSP lands (see `feat/audit-csp-report-only`), `X-Frame-Options`
  // is the only thing standing between the owner dashboard and a clickjacking
  // frame, so its absence has to fail loudly rather than quietly.
  it('ships no Content-Security-Policy at all yet, in either mode', () => {
    expect(headerValue('Content-Security-Policy')).toBeUndefined();
    expect(headerValue('Content-Security-Policy-Report-Only')).toBeUndefined();
    expect(headerValue('X-Frame-Options')).toBe('DENY');
  });
});
