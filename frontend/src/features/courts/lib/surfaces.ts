import type { Court, Sport } from '@/shared/types/api.types';

export type CourtSurface = NonNullable<Court['surface']>;

/** Every surface the API knows, in the order the form lists them. */
export const COURT_SURFACES: readonly CourtSurface[] = [
  'synthetic_grass',
  'natural_grass',
  'clay',
  'concrete',
  'parquet',
  'synthetic_floor',
  'sand',
  'rubber',
];

/**
 * Which surfaces each sport can be played on. Mirrors the server's
 * `backend/internal/courts/surfaces.go`, which is the authority: it refuses a
 * surface outside its sport's list with a 422. This copy only keeps the form
 * from offering a choice the server would reject.
 */
export const SURFACES_BY_SPORT: Record<Sport, readonly CourtSurface[]> = {
  soccer: ['synthetic_grass', 'natural_grass', 'concrete', 'parquet', 'synthetic_floor', 'sand', 'rubber'],
  padel: ['synthetic_grass', 'concrete'],
  tennis: ['synthetic_grass', 'natural_grass', 'clay', 'concrete', 'synthetic_floor'],
  hockey: ['synthetic_grass'],
  basketball: ['concrete', 'parquet', 'synthetic_floor'],
  volleyball: ['parquet', 'synthetic_floor', 'sand'],
  pickleball: ['concrete', 'synthetic_floor'],
};

export function surfacesForSport(sport: Sport): readonly CourtSurface[] {
  return SURFACES_BY_SPORT[sport];
}

export function isSurfaceAllowed(sport: Sport, surface: string): surface is CourtSurface {
  return (surfacesForSport(sport) as readonly string[]).includes(surface);
}
