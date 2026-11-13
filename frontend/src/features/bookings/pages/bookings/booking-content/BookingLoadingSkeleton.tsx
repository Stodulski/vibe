import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';
import {
  GRID_HEIGHT_PX,
  GUTTER_CLASS,
  HEADER_HEIGHT_PX,
  HOUR_HEIGHT_PX,
} from '@/features/bookings/components/booking-calendar/gridLayout';

const t = ES_AR;

const COURT_COLUMNS = 3;
// The real gutter labels hours 01:00..23:00; 24 rows of one hour cover the day.
const HOUR_ROWS = GRID_HEIGHT_PX / HOUR_HEIGHT_PX;

/**
 * Mirrors `CourtTimeGrid`: no card, a hour gutter on the left, a court-name
 * header row (36px) and a full-day column area (1344px). Columns share the
 * width but never drop below 200px, like the real grid, so a phone shows
 * roughly one court and the rest is clipped.
 */
export function BookingLoadingSkeleton() {
  return (
    <div className="relative -mr-3 flex overflow-hidden sm:mr-0" role="status" aria-label={t.common.loading}>
      <div className={GUTTER_CLASS}>
        <div style={{ height: `${String(HEADER_HEIGHT_PX)}px` }} />
        <div style={{ height: `${String(GRID_HEIGHT_PX)}px` }}>
          {Array.from({ length: HOUR_ROWS }, (_, i) => (
            <div key={i} className="flex items-start" style={{ height: `${String(HOUR_HEIGHT_PX)}px` }}>
              <Skeleton className="h-3 w-8 rounded-lg" />
            </div>
          ))}
        </div>
      </div>
      <div className="flex min-w-0 flex-1">
        {Array.from({ length: COURT_COLUMNS }, (_, c) => (
          <div key={c} className="min-w-[200px] flex-1">
            <div className="flex items-center justify-center px-2" style={{ height: `${String(HEADER_HEIGHT_PX)}px` }}>
              <Skeleton className="h-5 w-20 rounded-full" />
            </div>
            <div className="border-border-subtle border-l" style={{ height: `${String(GRID_HEIGHT_PX)}px` }}>
              {Array.from({ length: HOUR_ROWS }, (_, r) => (
                <div
                  key={r}
                  className="border-border-subtle border-t"
                  style={{ height: `${String(HOUR_HEIGHT_PX)}px` }}
                />
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
