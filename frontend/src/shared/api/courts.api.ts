import api, { withSignal } from '@/shared/lib/ky';
import type { CourtWithPrices } from '@/shared/types/api.types';
import { parseWith } from '@/shared/lib/apiParse';
import { courtsListEnvelopeSchema } from '@/shared/schemas/court.schema';

/**
 * The complex's courts with their prices — the one courts endpoint
 * `useCourts` (`shared/hooks/useCourts.ts`) needs.
 *
 * Lives in `shared/` (not only `features/courts/api/courts.api.ts`, which
 * still owns every other courts endpoint and delegates its own `list` to
 * this) because the schedule tab in `features/complex` checks the new opening
 * hours against these prices, and features never import from one another —
 * same move as `shared/api/cashSession.api.ts`.
 */
export function listCourts(complexId: string, signal?: AbortSignal): Promise<{ courts: CourtWithPrices[] }> {
  return api
    .get(`complexes/${complexId}/courts`, withSignal(signal))
    .json()
    .then(parseWith(courtsListEnvelopeSchema, 'courtsApi.list'));
}
