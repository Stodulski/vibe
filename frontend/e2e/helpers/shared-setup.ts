import { createApiHelper, ApiHelper } from './api.helper';
import type { ApiComplex, ApiCourt } from './api.helper';

const SHARED_SLUG = 'complejo-e2e-shared';

interface SharedSetupResult {
  complexId: string;
  complexSlug: string;
  courtId: string;
  courtName: string;
}

// Named interface instead of a `typeof <let-bound-to-null>` query for the
// promise cache's type: `typeof` on a `let` binding whose declared value is
// the literal `null` narrows to `null` at this lexical point (a real TS
// control-flow quirk, reproduced independently of this refactor), which
// silently collapsed every subsequent usage to `Promise<null>` and made the
// eslint type-aware rules see `error`/`any` downstream.
//
// A resolved promise already caches its value and resolves synchronously on
// every subsequent `await`, so this promise cache alone is sufficient
// memoization; no separate `_sharedSetup` result cache is needed.
let _setupPromise: Promise<SharedSetupResult> | null = null;

/**
 * Returns a shared complex+court setup, creating it only once across all authenticated specs.
 * This avoids hitting the 4-complex-per-owner limit.
 */
export async function getSharedSetup(): Promise<SharedSetupResult & { api: ApiHelper }> {
  const api = await createApiHelper();

  _setupPromise ??= (async () => {
    // Check if we already have a complex
    try {
      const { complex, court } = await api.setupFullComplex({
        name: 'Complejo E2E Shared',
        slug: SHARED_SLUG,
      });
      return {
        complexId: complex.id,
        complexSlug: SHARED_SLUG,
        courtId: court.id,
        courtName: court.name,
      };
    } catch (e: unknown) {
      // Only a taken slug means "another worker already built it". Any other
      // failure is a real one and must surface as itself.
      if (!(e instanceof Error) || !e.message.includes('slug_taken')) throw e;

      // Playwright restarts the worker after a failure and this promise cache
      // is per worker, so a retry lands here with the complex already in place.
      // Look it up through the typed public `ApiHelper.get()` accessor rather
      // than the class's private request fields.
      const { complexes } = await api.get<{ complexes: ApiComplex[] }>('/complexes');
      const existing = complexes.find((c) => c.slug === SHARED_SLUG);
      if (!existing) {
        throw new Error(
          `shared complex "${SHARED_SLUG}" is slug_taken but is not among this owner's ${String(complexes.length)} complexes`,
          { cause: e },
        );
      }

      // Re-create the court when a previous worker's setup died between
      // creating the complex and its court: reusing a court-less complex only
      // moves the failure into the first spec that needs a court.
      const { courts } = await api.get<{ courts: ApiCourt[] }>(`/complexes/${existing.id}/courts`);
      let court = courts[0];
      if (!court) {
        court = await api.createCourt(existing.id);
        await api.setCourtPrices(existing.id, court.id);
      }
      return {
        complexId: existing.id,
        complexSlug: existing.slug,
        courtId: court.id,
        courtName: court.name,
      };
    }
  })();

  const result = await _setupPromise;
  return { ...result, api };
}
