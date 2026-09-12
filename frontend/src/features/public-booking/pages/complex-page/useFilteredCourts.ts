import type { AvailabilityData, Sport } from '@/shared/types/api.types';

/** Courts from the current availability fetch, narrowed to the active sport filter. */
export function useFilteredCourts(availability: AvailabilityData | undefined, sportFilter: Sport | null) {
  if (!availability) return [];
  if (!sportFilter) return availability.courts;
  return availability.courts.filter((c) => c.sport === sportFilter);
}
