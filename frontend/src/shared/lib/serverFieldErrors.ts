import type { FieldValues, Path, UseFormReturn } from 'react-hook-form';
import { getProblem } from '@/shared/lib/ApiError';

/** The slice of a form this helper needs — any `useForm` return satisfies it. */
export type ServerErrorForm<T extends FieldValues> = Pick<UseFormReturn<T>, 'setError' | 'getValues'>;

export interface ApplyServerFieldErrorsOptions {
  /**
   * The field names the form will accept. Defaults to the keys of
   * `getValues()`, which is every registered field with a default value.
   *
   * `setError` on a name the form does not know is a silent no-op, so an
   * unmapped error must be recognized here and routed to `root` instead —
   * otherwise the server's reason for refusing the submit disappears.
   */
  fields?: Iterable<string> | undefined;
  /** Called for each field that received an error, e.g. to open a collapsed group. */
  onField?: ((field: string) => void) | undefined;
  /** `root` message when the server refused without naming a field the form owns. */
  fallback?: string | undefined;
}

/**
 * Put a rejected submit back on the form it came from.
 *
 * Every form used to do this itself, or — in thirteen of sixteen cases — not
 * at all: the 422 became a toast, floating away from the input that caused
 * it. Two near-identical private copies of this function existed
 * (`useComplexForm`, `GoogleCompleteForm`); this is the one both now call.
 *
 * Reads the normalized {@link Problem} from `ApiError` — its `errors[]`,
 * already mapped from problem+json's `field`/`pointer` addressing.
 *
 * Anything that cannot land on a field lands on `root`: React Hook Form
 * keeps `root` out of `getValues()` and clears it on the next submit, so it
 * is the form's own "this attempt failed, and here is why" slot.
 *
 * Returns whether at least one error landed on a real field — callers use it
 * to decide whether a toast would be a duplicate of what is already on screen.
 */
export function applyServerFieldErrors<T extends FieldValues>(
  form: ServerErrorForm<T>,
  error: unknown,
  options: ApplyServerFieldErrorsOptions = {},
): boolean {
  const problem = getProblem(error);
  if (!problem) return false;

  const known = new Set(options.fields ?? Object.keys(form.getValues()));
  const unmapped: string[] = [];
  let applied = false;

  for (const { field, message } of problem.errors) {
    if (!known.has(field)) {
      unmapped.push(message);
      continue;
    }
    form.setError(field as Path<T>, { type: 'server', message });
    options.onField?.(field);
    applied = true;
  }

  // `||`, not `??`: each of these is a string that can be legitimately
  // empty (no unmapped errors, no detail, no title), and an empty one has
  // nothing to say — it must fall through to the next, not win.
  // eslint-disable-next-line @typescript-eslint/prefer-nullish-coalescing
  const rootMessage = unmapped.join('. ') || problem.detail || problem.title || options.fallback;
  if (rootMessage && !applied) {
    form.setError('root', { type: 'server', message: rootMessage });
  }

  return applied;
}
