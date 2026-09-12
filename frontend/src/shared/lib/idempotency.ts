import { useMutation } from '@tanstack/react-query';
import type { MutateOptions, UseMutationOptions, UseMutationResult } from '@tanstack/react-query';

/** A fresh `Idempotency-Key`. A UUID is 36 chars, well inside the backend's 64-char limit. */
export function newIdempotencyKey(): string {
  return crypto.randomUUID();
}

/**
 * A mutation's own variables, plus the key minted for the `mutate()` call
 * that produced them.
 *
 * Carrying the key *in the variables* is what makes it correct under
 * concurrency. React Query hands the same variables object to every retry of
 * one `mutate()` and never shares it between two of them, so the key is
 * automatically stable per attempt and distinct per call — with no state for
 * a second call to overwrite.
 *
 * The alternative, a ref on the hook, is broken: `onMutate` is async (it
 * awaits `cancelQueries`), so confirming payment on booking A and then on
 * booking B before A's request leaves would mint A's key, mint B's over it,
 * and then send *both* bodies under B's key. The backend answers the second
 * one 409 — same key, different body — which the public booking flow renders
 * as "ese turno ya fue reservado", a conflict that never happened.
 */
export type WithAttemptKey<TVars> = TVars & { attemptKey: string };

/**
 * `useMutation` for the endpoints the backend deduplicates on
 * `Idempotency-Key`: `POST /book`, `POST /complexes/:id/bookings`,
 * `confirm-payment` and `manual-refund`.
 *
 * Identical to `useMutation` from the caller's side — `mutate(variables)`
 * takes the same variables it always did — except that a fresh key is minted
 * per call and added to the variables the `mutationFn` receives. Destructure
 * `attemptKey` out there, so it reaches the header and never the request body
 * (outside production the backend validates bodies against the OpenAPI spec
 * and would refuse the extra field with a 400).
 */
export function useIdempotentMutation<TData, TError, TVars extends object, TContext>(
  options: UseMutationOptions<TData, TError, WithAttemptKey<TVars>, TContext>,
): UseMutationResult<TData, TError, TVars, TContext> {
  const mutation = useMutation(options);

  return {
    ...mutation,
    // The per-call callbacks are forwarded only when the caller passed some:
    // React Query's rest tuple declares that slot optional, and under
    // `exactOptionalPropertyTypes` handing it an explicit `undefined` is not
    // the same as omitting it.
    mutate: (variables: TVars, mutateOptions?: MutateOptions<TData, TError, TVars, TContext>) => {
      const keyed = { ...variables, attemptKey: newIdempotencyKey() };
      if (mutateOptions) mutation.mutate(keyed, mutateOptions);
      else mutation.mutate(keyed);
    },
    mutateAsync: (variables: TVars, mutateOptions?: MutateOptions<TData, TError, TVars, TContext>) => {
      const keyed = { ...variables, attemptKey: newIdempotencyKey() };
      return mutateOptions ? mutation.mutateAsync(keyed, mutateOptions) : mutation.mutateAsync(keyed);
    },
    // `UseMutationResult` is a discriminated union over `status`, which a
    // spread flattens — the fields are all present and correctly typed (the
    // inner `variables` is a `WithAttemptKey<TVars>`, which *is* a `TVars`),
    // but the literal no longer matches any single member of the union.
  } as UseMutationResult<TData, TError, TVars, TContext>;
}
