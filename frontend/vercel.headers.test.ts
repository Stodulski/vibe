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
 * The CSP goes out as `Content-Security-Policy-Report-Only` first (SEC-02):
 * a missing origin shows up as a report instead of a blank owner dashboard,
 * and the switch to the enforcing header is a later, separate deploy. The
 * rest of the headers (SEC-03 / DEP-02) enforce immediately — none of them
 * can break a page the way a wrong CSP can.
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

/** One CSP directive's source list, e.g. `script-src` -> `"'self' https://x"`. */
function cspDirective(name: string): string | undefined {
  const csp = headerValue('Content-Security-Policy-Report-Only') ?? '';
  return csp
    .split(';')
    .map((part) => part.trim())
    .find((part) => part === name || part.startsWith(`${name} `))
    ?.slice(name.length)
    .trim();
}

/**
 * The Sentry security endpoint every `report-uri`/`report-to` pair must
 * resolve to now that the Sentry projects exist (SEC-02 follow-up).
 *
 * All three path segments (org id, region, project id) and the query
 * parameter (the DSN's public key) are public by design — the DSN itself is
 * inlined in the production bundle for `Sentry.init({ dsn })` to read — so
 * this is not a secret pattern, only a shape check that catches a typo'd
 * project id or a copy-pasted org from the wrong Sentry account.
 */
const REPORT_URI_PATTERN = /^https:\/\/o\d+\.ingest(\.us)?\.sentry\.io\/api\/\d+\/security\/\?sentry_key=[a-f0-9]{32}$/;

describe('vercel.json security headers', () => {
  it('applies a rule to every path', () => {
    expect(globalRule).toBeDefined();
  });

  it.each([
    ['Content-Security-Policy-Report-Only', undefined],
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

  it('ships the CSP in Report-Only mode, not enforcing', () => {
    expect(headerValue('Content-Security-Policy')).toBeUndefined();
    expect(headerValue('Content-Security-Policy-Report-Only')).toBeTruthy();
  });

  it("never allows 'unsafe-inline' or 'unsafe-eval' in script-src", () => {
    const scriptSrc = cspDirective('script-src');
    expect(scriptSrc).toBeDefined();
    expect(scriptSrc).not.toContain("'unsafe-inline'");
    expect(scriptSrc).not.toContain("'unsafe-eval'");
  });

  it('locks down the directives an injected document cannot be allowed to use', () => {
    expect(cspDirective('default-src')).toBe("'self'");
    expect(cspDirective('base-uri')).toBe("'self'");
    expect(cspDirective('object-src')).toBe("'none'");
    // Belt and braces with X-Frame-Options above: modern browsers honour
    // `frame-ancestors`, older ones only the header.
    expect(cspDirective('frame-ancestors')).toBe("'none'");
  });

  it('allows form-action to submit to Google, for GIS redirect mode', () => {
    // `google.accounts.id.initialize({ ux_mode: 'redirect', login_uri })` in
    // GoogleSignInButton.tsx makes Google POST the credential back to our
    // own `login_uri` — that leg is documented in this repo (see the comment
    // on `callback` in src/shared/lib/googleIdentity.ts) and needs nothing
    // here, since `form-action` only restricts forms our own document
    // submits, not requests arriving at it.
    //
    // What is NOT determinable from this repo is the *outbound* leg: whether
    // clicking the rendered GIS button navigates the browser directly, or
    // whether Google's opaque `gsi/client` script (loaded from
    // accounts.google.com, not vendored or observable here) builds a
    // same-document `<form>` and submits it to accounts.google.com. If it
    // does the latter, `form-action 'self'` would silently block sign-in the
    // day this policy is switched from Report-Only to enforcing — a much
    // worse failure than an over-broad allowance now, while it only ever
    // generates a report. So this is added defensively rather than proven
    // from source; a real preview deployment's Report-Only console output
    // (clicking "Continuar con Google" end to end) is the way to actually
    // confirm which leg fires, if any.
    expect(cspDirective('form-action')).toBe("'self' https://accounts.google.com");
  });
});

/**
 * Every third-party origin the CSP allows, audited against the code on this
 * branch (not the 2026-09-12 draft) — one assertion per origin, each citing
 * the file that justifies it. A Report-Only policy that allows everything
 * reports nothing useful, so an origin with no citation here should not be
 * in `vercel.json` either.
 */
describe('vercel.json CSP third-party origins', () => {
  it.each([
    // Cloudflare Turnstile: `@marsidev/react-turnstile` (wrapping Cloudflare's
    // own widget script) — src/shared/components/common/TurnstileField.tsx.
    ['script-src', 'https://challenges.cloudflare.com'],
    ['frame-src', 'https://challenges.cloudflare.com'],
    // Google Identity Services, redirect mode. The script is loaded from
    // exactly this path (SCRIPT_SRC) — src/shared/lib/googleIdentity.ts —
    // and Google's own CSP guidance for GIS scopes script-src/frame-src to
    // the same `/gsi/` path rather than the whole origin.
    ['script-src', 'https://accounts.google.com/gsi/client'],
    ['frame-src', 'https://accounts.google.com/gsi/'],
    // The rendered GIS button pulls its own stylesheet from this path —
    // Google's CSP guidance's fourth source, alongside script/frame/connect —
    // src/features/auth/components/GoogleSignInButton.tsx.
    ['style-src', 'https://accounts.google.com/gsi/style'],
    // OpenStreetMap tiles — the `TileLayer` url in
    // src/features/public-booking/components/ComplexMap.tsx. Leaflet's own
    // marker icons are bundled assets (see the comment in that file), not a
    // CDN fetch, so no unpkg/CDN origin belongs here.
    ['img-src', 'https://*.tile.openstreetmap.org'],
    // Complex logo/cover images: `public_url` is whatever R2_PUBLIC_URL the
    // backend is configured with — either R2's own public dev domain or a
    // custom domain under vibe.com.ar (backend/internal/platform/config) —
    // rendered by src/features/complex/components/ImageUpload.tsx and
    // src/features/public-booking/components/ComplexHeader.tsx.
    ['img-src', 'https://*.r2.dev'],
    ['img-src', 'https://*.vibe.com.ar'],
    // The app's own API, on a different origin than the SPA in production
    // (VITE_API_URL is absolute there) — src/shared/lib/ky.ts.
    ['connect-src', 'https://api.vibe.com.ar'],
    // The landing's published MercadoPago fee schedule, fetched directly
    // (not through `ky`, deliberately no cookies) —
    // src/features/complex/api/mpFees.api.ts.
    ['connect-src', 'https://vibe.com.ar'],
    // Presigned R2 PUT upload: the browser does the upload itself, straight
    // to R2's S3-compatible API — `uploadApi.uploadToR2` in
    // src/features/complex/api/upload.api.ts. (R2's public dev domain, used
    // for the resulting `public_url` above, is never fetched from the
    // browser and so is not repeated here.)
    ['connect-src', 'https://*.r2.cloudflarestorage.com'],
    // Google's own CSP guidance for GIS also lists connect-src under
    // `/gsi/`, alongside script/frame/style — belt-and-braces for whatever
    // the loaded accounts.google.com script itself talks to.
    ['connect-src', 'https://accounts.google.com/gsi/'],
  ])('allows %s to reach %s', (directive, origin) => {
    expect(cspDirective(directive)).toContain(origin);
  });

  it('reaches the Sentry SDK at exactly its DSN host, not a region wildcard', () => {
    // src/shared/lib/sentry.ts passes the DSN straight to `Sentry.init`; the
    // SDK only ever talks to the ingest host baked into that DSN, so the
    // broader `*.ingest.sentry.io` / `*.ingest.us.sentry.io` wildcards the
    // 2026-09-12 draft carried (written before the DSN existed) reported
    // nothing a narrower, exact host doesn't also cover.
    expect(cspDirective('connect-src')).toContain('https://o4511023559868416.ingest.us.sentry.io');
    expect(cspDirective('connect-src')).not.toMatch(/\*\.ingest/);
  });

  it('does not carry origins the app never talks to from the browser', () => {
    const imgSrc = cspDirective('img-src') ?? '';
    const connectSrc = cspDirective('connect-src') ?? '';
    // Every @font-face in src/styles/globals.css is a same-origin
    // `/fonts/*.woff2` file — no `data:`-embedded or Google-hosted font.
    expect(cspDirective('font-src')).toBe("'self'");
    // No `googleusercontent.com` avatar is ever rendered (no
    // `googleusercontent`/`avatar_url`/`picture` reference in src/) — GIS
    // draws its own button, this app never shows the Google profile photo.
    expect(imgSrc).not.toContain('googleusercontent.com');
    // No <img>, canvas or CSS in this app resolves to a blob: URL — the only
    // `URL.createObjectURL` call is the report-export download link
    // (src/features/dashboard/pages/reports/useReportExport.ts), which is an
    // `<a download>` navigation, not an image/fetch destination.
    expect(imgSrc).not.toContain('blob:');
    // R2's public domain is an img-src concern (rendering `public_url`), not
    // a connect-src one — nothing in src/ fetches it directly.
    expect(connectSrc).not.toContain('*.r2.dev');
  });

  it('scopes MercadoPago to full navigations that need no CSP source', () => {
    // `mpAuth.ts` and `useConfirmBookingSubmit.ts` both hand MercadoPago URLs
    // to `window.location.href` / an anchor's `href` — top-level navigations,
    // which no CSP fetch directive governs. No MercadoPago script, SDK or
    // iframe is loaded in the browser (no `sdk.mercadopago`/`<iframe>` in
    // src/), so no `mercadopago.com` origin belongs in this policy at all.
    const csp = headerValue('Content-Security-Policy-Report-Only') ?? '';
    expect(csp).not.toContain('mercadopago.com');
  });
});

/**
 * The reporting endpoint itself: now a real Sentry security endpoint instead
 * of the 2026-09-12 placeholder, so violations land somewhere instead of
 * going into the void. Still Report-Only (see above) — enforcing it is a
 * later, separate deploy after a week of reports.
 */
describe('vercel.json CSP reporting endpoint', () => {
  it('points report-uri and report-to at a real Sentry security endpoint', () => {
    const reportUri = cspDirective('report-uri') ?? '';
    expect(reportUri).toMatch(REPORT_URI_PATTERN);
    expect(cspDirective('report-to')).toBe('csp-endpoint');
  });

  it('declares the same endpoint on Reporting-Endpoints, for report-to', () => {
    const reportingEndpoints = headerValue('Reporting-Endpoints') ?? '';
    const reportUri = cspDirective('report-uri');
    expect(reportingEndpoints).toBe(`csp-endpoint="${String(reportUri)}"`);
  });
});
