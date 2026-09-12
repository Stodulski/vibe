import { ES_AR } from '@/shared/i18n/es_AR';
import type { DurationMinutes } from '@/shared/types/api.types';

const t = ES_AR;

/**
 * The durations offered for a free slot, already filtered to the ones that fit.
 *
 * Pointing at one previews it on the grid behind, so the length is shown where
 * it will actually land instead of only as a number. Focus reports it too, so
 * the preview follows arrow-key navigation and not just the mouse.
 */
export function DurationChoices({
  durations,
  onPick,
  onPreview,
}: {
  durations: DurationMinutes[];
  onPick: (duration: DurationMinutes) => void;
  onPreview: (duration: DurationMinutes | null) => void;
}) {
  return (
    <div
      className="flex flex-col gap-1"
      onMouseLeave={() => {
        onPreview(null);
      }}
    >
      {durations.map((duration) => (
        <button
          key={duration}
          type="button"
          onClick={() => {
            onPick(duration);
          }}
          onMouseEnter={() => {
            onPreview(duration);
          }}
          onFocus={() => {
            onPreview(duration);
          }}
          onBlur={() => {
            onPreview(null);
          }}
          className="score-text border-border-subtle text-text-primary hover:border-primary-400 hover:text-primary-300 focus-visible:border-primary-400 focus-visible:text-primary-300 w-full rounded-md border px-3 py-2 text-center text-xs font-semibold transition-colors focus-visible:outline-none"
        >
          {duration} {t.bookings.minutesShort}
        </button>
      ))}
    </div>
  );
}
