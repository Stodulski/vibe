import { ES_AR } from '@/shared/i18n/es_AR';
import { minutesToPx } from './gridLayout';

const t = ES_AR;

/** A 1px horizontal line across every court column at the current minute, with a dot marking its left end. */
export function NowLine({ nowMin }: { nowMin: number }) {
  return (
    <div
      role="img"
      aria-label={t.bookings.currentTime}
      title={t.bookings.currentTime}
      className="bg-error-text pointer-events-none absolute inset-x-0 z-10 h-px"
      style={{ top: `${String(minutesToPx(nowMin))}px` }}
    >
      <div className="bg-error-text absolute -top-[3px] -left-[3px] size-[7px] rounded-full" />
    </div>
  );
}
