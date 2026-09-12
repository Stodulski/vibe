import * as Sentry from '@sentry/react';
import { useEffect } from 'react';
import { createRoutesFromChildren, matchRoutes, useLocation, useNavigationType } from 'react-router-dom';

const REDACTED_EMAIL = '[redacted-email]';
const REDACTED_PHONE = '[redacted-phone]';
const REDACTED_VALUE = '[redacted]';

const EMAIL_PATTERN = /[\w.+-]+@[\w-]+\.[a-zA-Z]{2,}/g;
// A run of 10-15 digits, optionally led by `+` and with spaces/dashes as
// internal separators — long enough to stay clear of 8-digit dates or short
// ids while covering both the app's E.164 numbers (+549...) and the raw
// local format the register form collects (see phoneField in validations.ts).
const PHONE_PATTERN = /\+?\d[\d\s-]{8,14}\d/g;

// Query/body param names that carry a session or verification secret rather
// than data about the request itself — a password-reset or email-verify
// link, a CSRF header echoed into an error, an OAuth callback.
const SENSITIVE_PARAMS = ['token', 'code', 'csrf', 'csrf_token', 'access_token', 'refresh_token'];

function scrubText(value: string): string {
  return value.replace(EMAIL_PATTERN, REDACTED_EMAIL).replace(PHONE_PATTERN, REDACTED_PHONE);
}

function isSensitiveParam(key: string): boolean {
  return SENSITIVE_PARAMS.includes(key.toLowerCase());
}

/**
 * `URLSearchParams` decodes a value while iterating its entries and
 * re-encodes it on `toString()`/`set()` — scrubbing has to happen on that
 * decoded value, in between, or an email like `juan@example.com` (encoded
 * as `juan%40example.com` in the raw query string) never matches
 * {@link EMAIL_PATTERN}'s literal `@`.
 */
function scrubQueryString(query: string): string {
  const params = new URLSearchParams(query);
  for (const [key, value] of [...params.entries()]) {
    params.set(key, isSensitiveParam(key) ? REDACTED_VALUE : scrubText(value));
  }
  return params.toString();
}

function scrubUrl(url: string): string {
  try {
    const parsed = new URL(url);
    parsed.search = scrubQueryString(parsed.search.replace(/^\?/, ''));
    // A further pass over the whole URL, for PII outside the query string
    // (e.g. an email in the path) — harmless for the query string itself,
    // since a redacted value contains no `@` or long digit run to rematch.
    return scrubText(parsed.toString());
  } catch {
    // Not a parseable absolute URL (a relative path, an already-malformed
    // string) — still worth scrubbing as plain text.
    return scrubText(url);
  }
}

/**
 * `beforeSend`: strips emails, phone numbers and session/verification
 * secrets from an event before it leaves the browser.
 *
 * `sendDefaultPii: false` (below) only stops the SDK from attaching things
 * like cookies or the client IP on its own — it says nothing about values
 * the app itself puts into a URL, an error message or a breadcrumb: a
 * `?token=...` reset link, an email echoed back in a validation error, a
 * phone number in a booking error. Exported for direct unit testing.
 */
export function scrubEvent(event: Sentry.ErrorEvent): Sentry.ErrorEvent {
  const { request } = event;
  if (request?.url) {
    request.url = scrubUrl(request.url);
  }
  if (request && typeof request.query_string === 'string') {
    request.query_string = scrubQueryString(request.query_string);
  }
  for (const value of event.exception?.values ?? []) {
    if (value.value) value.value = scrubText(value.value);
  }
  return event;
}

/** `beforeBreadcrumb`: the same scrub, for the trail leading up to an event. Exported for direct unit testing. */
export function scrubBreadcrumb(breadcrumb: Sentry.Breadcrumb): Sentry.Breadcrumb {
  const url: unknown = breadcrumb.data?.url;
  if (typeof url === 'string' && breadcrumb.data) {
    breadcrumb.data.url = scrubUrl(url);
  }
  if (breadcrumb.message) breadcrumb.message = scrubText(breadcrumb.message);
  return breadcrumb;
}

export function initSentry() {
  const dsn = import.meta.env.VITE_SENTRY_DSN;
  if (!dsn) return;

  Sentry.init({
    dsn,
    environment: import.meta.env.MODE,
    // The exact commit this bundle was built from (see vite.config.ts's
    // `define`), so an event in Sentry can be traced back to a deploy.
    release: APP_RELEASE,
    // Nothing beyond what the app explicitly attaches (see scrubEvent/
    // scrubBreadcrumb above) — no automatic cookies, no client IP.
    sendDefaultPii: false,
    beforeSend: scrubEvent,
    beforeBreadcrumb: scrubBreadcrumb,
    integrations: [
      // Router-aware tracing: transactions are named after the route
      // pattern ("/:slug/book/confirm"), not the raw pathname, and render
      // errors get tagged with which route they happened on. The
      // version-suffixed `reactRouterV7BrowserTracingIntegration` is
      // deprecated in favor of this version-agnostic one — same options,
      // "works with React Router v6+" per its own docs.
      Sentry.reactRouterBrowserTracingIntegration({
        useEffect,
        useLocation,
        useNavigationType,
        createRoutesFromChildren,
        matchRoutes,
      }),
      Sentry.replayIntegration(),
    ],
    tracesSampleRate: 0.1,
    // Replay is off for ordinary sessions and captures only the minute
    // before a reported error, so it costs nothing until something breaks.
    replaysSessionSampleRate: 0,
    replaysOnErrorSampleRate: 1.0,
  });

  // Tag every later event with the PWA/service-worker state at boot, so a
  // report says whether it came from an installed standalone app and which
  // build's worker was controlling the tab.
  if (typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
    Sentry.setTag('pwa_standalone', String(window.matchMedia('(display-mode: standalone)').matches));
  }
  if (typeof navigator !== 'undefined') {
    // lib.dom types `Navigator.serviceWorker` as always present, but it's
    // absent on a browser too old to support service workers at all and on
    // the test DOM this file's own tests run against (happy-dom) — worth
    // guarding despite the type.
    // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition
    Sentry.setTag('sw_version', navigator.serviceWorker?.controller ? APP_RELEASE : 'none');
  }
}
