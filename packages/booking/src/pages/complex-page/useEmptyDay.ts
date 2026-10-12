import { useQueries } from '@tanstack/react-query';
import { usePublicBookingApi } from '../../api/context';
import { queryKeys } from '../../lib/queryKeys';
import type { AvailabilityData, CourtAvailability, DurationMinutes } from '../../api/types';
import { classifyEmptyDay, hasFreeSlot, type EmptyDay } from './emptyDay';
import { DURATIONS } from './useComplexPageState';

/**
 * Whether the day has a free slot at a duration the visitor did not choose.
 * Asked only while the chosen duration and the sport filter both leave nothing
 * free, which is the one moment the answer decides between "filtered" and
 * "full". A failed check answers `true`: the day is then never called full on
 * a guess.
 */
function useOtherDurationsFree(
  slug: string,
  date: string,
  duration: DurationMinutes,
  enabled: boolean,
): boolean | 'checking' {
  const api = usePublicBookingApi();
  const results = useQueries({
    queries: DURATIONS.filter((other) => other !== duration).map((other) => ({
      queryKey: queryKeys.availability.bySlugAndDate(slug, date, other),
      queryFn: ({ signal }: { signal: AbortSignal }) => api.getAvailability(slug, date, other, signal),
      select: (data: { availability: AvailabilityData }): CourtAvailability[] => data.availability.courts,
      enabled,
      throwOnError: false,
      staleTime: 30 * 1000,
    })),
  });

  if (!enabled) return false;
  if (results.some((result) => result.isError)) return true;
  if (results.some((result) => result.data === undefined)) return 'checking';
  return results.some((result) => hasFreeSlot(result.data ?? []));
}

interface UseEmptyDayInput {
  slug: string;
  /** The day the grid shows, as `YYYY-MM-DD`. */
  dateStr: string;
  /** The duration the availability was requested with. */
  duration: DurationMinutes;
  /** The availability of the day at `duration`, before any sport filter. */
  availability: AvailabilityData | undefined;
  /** The courts the grid shows, after the sport filter. */
  shownCourts: CourtAvailability[];
}

/** Why the grid is empty, or `null` when it has free slots or the day is closed. */
export function useEmptyDay({ slug, dateStr, duration, availability, shownCourts }: UseEmptyDayInput): EmptyDay | null {
  const isOpen = availability?.is_open ?? false;
  const shownHasFreeSlot = hasFreeSlot(shownCourts);
  const unfilteredHasFreeSlot = availability ? hasFreeSlot(availability.courts) : false;
  const needsOtherDurations = !!slug && isOpen && !shownHasFreeSlot && !unfilteredHasFreeSlot;

  const otherDurations = useOtherDurationsFree(slug, dateStr, duration, needsOtherDurations);
  if (!isOpen) return null;

  return classifyEmptyDay({ shownHasFreeSlot, unfilteredHasFreeSlot, otherDurations });
}
