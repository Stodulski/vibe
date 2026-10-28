import { useRef } from 'react';
import { useMutation } from '@tanstack/react-query';
import type { MutateOptions, UseMutationOptions, UseMutationResult } from '@tanstack/react-query';

/** A fresh `Idempotency-Key`. A UUID is 36 chars, well inside the backend's 64-char limit. */
function newIdempotencyKey(): string {
  return crypto.randomUUID();
}

/**
 * Whether the server answered an attempt with an HTTP status. ky's `HTTPError`
 * carries the `Response`, and so does the `ApiError` the frontend client
 * rethrows (it extends `HTTPError`); a timeout, an abort or a network failure
 * carries none. packages/ui does not depend on ky, so this reads the shape of
 * the error rather than its class.
 */
function gotHttpAnswer(error: unknown): boolean {
  if (!(error instanceof Error)) return false;
  return typeof (error as { response?: { status?: unknown } }).response?.status === 'number';
}

/** The `detail` the backend sends while the first request under a key is still running (backend/internal/middleware/idempotency.go). */
const IN_PROGRESS_DETAIL = 'a request with this Idempotency-Key is still in progress, retry shortly';

/**
 * Whether the server refused a repeated key because its first request has not
 * finished. That key is still live, so the retry must reuse it. Generic
 * conflicts share the 409 and the problem type, so the detail is what names
 * this case; the status alone proves nothing.
 */
function isStillInProgress(error: unknown): boolean {
  if (!(error instanceof Error)) return false;
  const { response, data } = error as { response?: { status?: unknown }; data?: unknown };
  if (response?.status !== 409 || typeof data !== 'object' || data === null) return false;
  const { type, detail } = data as { type?: unknown; detail?: unknown };
  return typeof type === 'string' && type.endsWith('/conflict') && detail === IN_PROGRESS_DETAIL;
}

/**
 * A mutation's own variables, plus the key of the submission they belong to.
 *
 * Carrying the key *in the variables* is what makes it correct under
 * concurrency: each `mutationFn` call carries its own key, so two calls in
 * flight can never send each other's. The hook chooses that key synchronously
 * inside `mutate()`, before React Query's async `onMutate` runs.
 *
 * The alternative, reading a ref from inside `mutationFn` or `onMutate`, is
 * broken: `onMutate` is async (it awaits `cancelQueries`), so confirming
 * payment on booking A and then on booking B before A's request leaves would
 * pick A's key, pick B's over it, and then send *both* bodies under B's key.
 * The backend answers the second one 409 — same key, different body — which
 * the public booking flow renders as "ese turno ya fue reservado", a conflict
 * that never happened.
 */
export type WithAttemptKey<TVars> = TVars & { attemptKey: string };

/**
 * `useMutation` for the endpoints the backend deduplicates on
 * `Idempotency-Key`: `POST /book`, `POST /complexes/:id/bookings`,
 * `confirm-payment` and `manual-refund`.
 *
 * Identical to `useMutation` from the caller's side — `mutate(variables)`
 * takes the same variables it always did. The key is per submission, not per
 * call: a retry of the same payload after a failure that gave no HTTP answer (a
 * client timeout, a network error) reuses the key, so the backend replays or
 * joins the first attempt instead of creating a second booking. A changed
 * payload, a success, or an HTTP error answer gets a new key, except the
 * "still in progress" refusal, which keeps it: the first request is still
 * running under that key. The key is added
 * to the variables the `mutationFn` receives.
 * Destructure `attemptKey` out there, so it reaches the header and never the
 * request body (outside production the backend validates bodies against the
 * OpenAPI spec and would refuse the extra field with a 400).
 */
export function useIdempotentMutation<TData, TError, TVars extends object, TContext>(
  options: UseMutationOptions<TData, TError, WithAttemptKey<TVars>, TContext>,
): UseMutationResult<TData, TError, TVars, TContext> {
  // The submission a retry would repeat: its payload and the key it was sent
  // with. Written only synchronously in `mutate()`, and cleared by a success or
  // an HTTP answer that still holds the same key, so a newer submission is never cleared.
  const pending = useRef<{ payload: string; key: string } | null>(null);

  const mutation = useMutation({
    ...options,
    onSuccess: (data, variables, onMutateResult, context) => {
      if (pending.current?.key === variables.attemptKey) pending.current = null;
      return options.onSuccess?.(data, variables, onMutateResult, context);
    },
    onError: (error, variables, onMutateResult, context) => {
      // An HTTP answer settles the attempt: the server has decided, and a
      // repeated key would replay that decision. Two cases keep the key: no
      // answer at all (a timeout or a network failure), and the refusal saying
      // the first request under the key is still running.
      const settled = gotHttpAnswer(error) && !isStillInProgress(error);
      if (settled && pending.current?.key === variables.attemptKey) pending.current = null;
      return options.onError?.(error, variables, onMutateResult, context);
    },
  });

  const withAttemptKey = (variables: TVars): WithAttemptKey<TVars> => {
    const payload = JSON.stringify(variables);
    const current = pending.current;
    if (current?.payload === payload) return { ...variables, attemptKey: current.key };
    const fresh = { payload, key: newIdempotencyKey() };
    pending.current = fresh;
    return { ...variables, attemptKey: fresh.key };
  };

  return {
    ...mutation,
    // The per-call callbacks are forwarded only when the caller passed some:
    // React Query's rest tuple declares that slot optional, and under
    // `exactOptionalPropertyTypes` handing it an explicit `undefined` is not
    // the same as omitting it.
    mutate: (variables: TVars, mutateOptions?: MutateOptions<TData, TError, TVars, TContext>) => {
      const keyed = withAttemptKey(variables);
      if (mutateOptions) mutation.mutate(keyed, mutateOptions);
      else mutation.mutate(keyed);
    },
    mutateAsync: (variables: TVars, mutateOptions?: MutateOptions<TData, TError, TVars, TContext>) => {
      const keyed = withAttemptKey(variables);
      return mutateOptions ? mutation.mutateAsync(keyed, mutateOptions) : mutation.mutateAsync(keyed);
    },
    // `UseMutationResult` is a discriminated union over `status`, which a
    // spread flattens — the fields are all present and correctly typed (the
    // inner `variables` is a `WithAttemptKey<TVars>`, which *is* a `TVars`),
    // but the literal no longer matches any single member of the union.
  } as UseMutationResult<TData, TError, TVars, TContext>;
}
