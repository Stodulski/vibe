import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { complexApi } from '../../api/complex.api';
import { queryKeys } from '@/shared/lib/queryKeys';

export type SlugState =
  | { status: 'idle' }
  | { status: 'checking' }
  | { status: 'free' }
  | { status: 'taken'; suggestion?: string | undefined };

const DEBOUNCE_MS = 400;

// Debounces the slug itself, not just the query: `debounced` starts as
// `undefined` (never a real slug) so the *first* mount also waits out
// `delayMs` before settling, the same as the effect this replaced — a fresh
// form showing its derived slug does not fire an immediate lookup the instant
// it appears.
function useDebouncedSlug(slug: string | undefined, delayMs: number): string | undefined {
  const [debounced, setDebounced] = useState<string | undefined>(undefined);
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebounced(slug);
    }, delayMs);
    return () => {
      clearTimeout(timer);
    };
  }, [slug, delayMs]);
  return debounced;
}

/**
 * Whether a public URL is still free, asked while it is being typed.
 *
 * A taken slug is the one error on this form that no amount of client-side
 * validation can predict — two owners can pick "club-norte" a second apart —
 * and the book names availability as one of the few cases worth checking as
 * you type rather than on submit.
 *
 * It matters MORE here than a username field usually does, because the slug is
 * derived from the name and hidden behind "Editar": most owners never look at
 * it. Without this the collision surfaces on submit, against a field they
 * never saw, after they filled in everything else.
 *
 * Debounced at 400ms via `useDebouncedValue`, and `useQuery` runs the actual
 * check — replacing the hand-rolled `setTimeout` + `AbortController` +
 * stale-response comparison this used to do by hand. That also buys caching
 * for free: typing "club-norte" -> "club-nort" -> "club-norte" answers the
 * last state instantly from cache instead of asking the server again.
 *
 * Skips `currentSlug`: a complex checking its own name is not a collision.
 */
export function useSlugAvailability(slug: string | undefined, currentSlug?: string): SlugState {
  const debouncedSlug = useDebouncedSlug(slug, DEBOUNCE_MS);
  const skip = !slug || slug === currentSlug;
  // The debounce has not settled on the current `slug` yet — still typing.
  const settled = debouncedSlug === slug;

  const { data, isFetching, isError } = useQuery({
    queryKey: queryKeys.complexes.slugAvailable(debouncedSlug ?? ''),
    // `enabled` guarantees `debouncedSlug` is a real string whenever this
    // runs; the guard just gives TypeScript the same guarantee without an
    // assertion.
    queryFn: ({ signal }) => {
      if (!debouncedSlug) return Promise.reject(new Error('slug-available query ran without a slug'));
      return complexApi.slugAvailable(debouncedSlug, signal);
    },
    enabled: !skip && !!debouncedSlug && settled,
    retry: false,
    staleTime: 60 * 1000,
  });

  if (skip) return { status: 'idle' };
  if (!settled || isFetching) return { status: 'checking' };
  // A failed check is not a taken slug. Say nothing and let the submit
  // answer — the server refuses a duplicate either way.
  if (isError || data?.slug !== slug) return { status: 'idle' };
  return data.available ? { status: 'free' } : { status: 'taken', suggestion: data.suggestion };
}
