import { HTTPError, NetworkError, TimeoutError } from 'ky';
import { BOOKING_MESSAGES } from '../messages';
import { ApiError, normalizeProblem, type Problem } from './ApiError';
import { ApiResponseError } from './apiParse';

// Browsers reject a fetch that never reached the server with a bare TypeError
// whose message depends on the runtime. Only these count as connectivity
// failures; any other TypeError is a programming defect and keeps the fallback.
const FETCH_FAILURE_MESSAGES = ['Failed to fetch', 'NetworkError when attempting to fetch resource', 'Load failed'];

function isFetchFailure(error: unknown): error is TypeError {
  return error instanceof TypeError && FETCH_FAILURE_MESSAGES.some((m) => error.message.startsWith(m));
}

/** Field errors first and joined, then `detail`, then `title`, then the fallback (RFC 9457 order). */
function problemMessage(problem: Problem, fallback: string): string {
  if (problem.errors.length > 0) return problem.errors.map((e) => e.message).join('. ');
  return problem.detail ?? (problem.title || fallback);
}

/**
 * The text to show for a failed booking request. Mirrors the app's
 * `getHttpErrorMessage`: a schema mismatch and a connection failure each get
 * their own copy whatever the caller's fallback says, and anything else reads
 * the problem body before falling back.
 */
export function getHttpErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiResponseError) return BOOKING_MESSAGES.common.invalidResponse;
  if (error instanceof ApiError) return problemMessage(error.problem, fallback);
  if (error instanceof HTTPError) return problemMessage(normalizeProblem(error.data, 0), fallback);

  if (error instanceof TimeoutError || error instanceof NetworkError || isFetchFailure(error)) {
    return BOOKING_MESSAGES.common.networkError;
  }
  // `typeof navigator.onLine === 'boolean'`: Node defines a `navigator` global
  // without `onLine`, which would otherwise read as offline.
  if (typeof navigator !== 'undefined' && typeof navigator.onLine === 'boolean' && !navigator.onLine) {
    return BOOKING_MESSAGES.common.networkError;
  }

  return fallback;
}
