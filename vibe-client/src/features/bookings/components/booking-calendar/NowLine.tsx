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
      className="pointer-events-none absolute inset-x-0 z-10 h-px bg-error-text"
      style={{ top: `${String(minutesToPx(nowMin))}px` }}
    >
      <div className="absolute -left-[3px] -top-[3px] size-[7px] rounded-full bg-error-text" />
    </div>
  );
}
