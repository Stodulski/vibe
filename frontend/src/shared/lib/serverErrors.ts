import { ES_AR } from '@/shared/i18n/es_AR';
import { getProblem } from '@/shared/lib/ApiError';

/**
 * Text for the error codes the API returns.
 *
 * The backend answers with a code, never a sentence, for anything a person
 * will read — see `internal/httpx/codes.go`. The wording lives here, with the
 * rest of the copy, so it exists in one place instead of two repositories.
 *
 * Keys are API contract: renaming one silently falls back to a generic
 * message, so they change only alongside the backend.
 */
const SERVER_ERROR_TEXT: Record<string, string> = {
  slug_taken: ES_AR.validation.server.slugTaken,
  deposit_percentage_over_100: ES_AR.validation.server.depositOver100,
  deposit_exceeds_price: ES_AR.validation.server.depositExceedsPrice,
  month_out_of_range: ES_AR.validation.server.monthOutOfRange,
  report_period_in_future: ES_AR.validation.server.reportPeriodInFuture,
  report_period_before_complex_existed: ES_AR.validation.server.reportPeriodBeforeComplex,
  report_export_too_large: ES_AR.dashboard.exportTooLarge,
  report_export_timed_out: ES_AR.dashboard.exportTimedOut,
};

/**
 * Translate one API error value.
 *
 * A known code becomes its Spanish text. Anything else is returned unchanged:
 * the backend still has messages that are prose rather than codes, and showing
 * an imperfect one beats replacing it with a generic that says less.
 */
export function translateServerError(value: string): string {
  return SERVER_ERROR_TEXT[value] ?? value;
}

/**
 * Extract the per-field validation errors out of a failed request.
 *
 * The backend answers a failed validation as problem+json's `errors[]` — one
 * `{field, message}` entry per invalid field, already read into
 * {@link Problem} by `ApiError`. Returns an empty object for any other
 * failure (no error at all, one that never reached the API, a 422 with no
 * field to blame), so callers can treat "no field errors" and "not a
 * validation failure" the same way.
 */
export function getFieldErrors(error: unknown): Record<string, string> {
  const problem = getProblem(error);
  if (!problem) return {};

  const fields: Record<string, string> = {};
  for (const { field, message } of problem.errors) {
    fields[field] = message;
  }
  return fields;
}

/**
 * Text for the `turnstile_token` field's own error codes.
 *
 * Kept out of {@link SERVER_ERROR_TEXT}: `required`/`invalid`/`unavailable`
 * are bare codes the server only ever attaches to this one field (see
 * `TURNSTILE_SECRET_KEY` in the server). Folding them into the global,
 * field-agnostic map would let some unrelated field's future `required`
 * code collide with this wording.
 */
const TURNSTILE_ERROR_TEXT: Record<string, string> = {
  required: ES_AR.auth.turnstileRequired,
  invalid: ES_AR.auth.turnstileInvalid,
  unavailable: ES_AR.auth.turnstileUnavailable,
};

/**
 * Extract and translate the `turnstile_token` field error out of a failed
 * request, if any. Built on {@link getFieldErrors} rather than a separate
 * parser — the raw code passes through `translateServerError` untouched (it
 * isn't in {@link SERVER_ERROR_TEXT}), then gets mapped here.
 */
export function getTurnstileError(error: unknown): string | undefined {
  const code = getFieldErrors(error).turnstile_token;
  return code ? (TURNSTILE_ERROR_TEXT[code] ?? code) : undefined;
}
