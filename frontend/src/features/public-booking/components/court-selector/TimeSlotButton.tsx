import { cn } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { COURT_TYPE_LABELS } from './constants';
import type { TimeOption } from './timeOptions';

const t = ES_AR;

interface TimeSlotButtonProps {
  option: TimeOption;
  isSelected: boolean;
  /**
   * Whether this venue has courts of more than one type. Decided once for the
   * grid: a club whose courts are all covered has nothing to warn anyone
   * about, and stamping "techada" on every hour would be a label that never
   * varies.
   */
  typesVary: boolean;
  onSelect: (option: TimeOption) => void;
}

/**
 * One bookable hour.
 *
 * Every button here is pressable — an hour with no free court never becomes a
 * TimeOption at all, so there is no disabled state to design.
 *
 * No price. The price belongs to a court, not to an hour: at a venue where the
 * covered court costs more, the hour could only say "desde", and the court
 * cards that follow say the exact figure. Forty-five buttons repeating the
 * same "$140" said nothing an hour needed to say.
 *
 * The count of free courts is deliberately absent except at one: a number
 * that does not change the decision is noise on forty-five buttons, but
 * "última cancha" changes whether someone books now or thinks about it.
 */
export function TimeSlotButton({ option, isSelected, typesVary, onSelect }: TimeSlotButtonProps) {
  const isLast = option.courts.length === 1;
  // Said only when this hour has narrowed to one type at a venue that has
  // several — the moment the answer stops being "whichever you like".
  const typeLabel = typesVary && option.soleType ? (COURT_TYPE_LABELS[option.soleType] ?? option.soleType) : null;

  return (
    <button
      data-slot-time={option.startTime}
      onClick={() => {
        onSelect(option);
      }}
      aria-pressed={isSelected}
      aria-label={`${option.startTime}${typeLabel ? `, ${typeLabel}` : ''}${isLast ? `, ${t.publicBooking.lastCourtLeft}` : ''}`}
      className={cn(
        'press-scale flex cursor-pointer flex-col items-center rounded-xl border px-2 py-3 transition-colors duration-200',
        isSelected
          ? 'border-primary-500 bg-primary-500/10 shadow-brand ring-primary-500/30 ring-1'
          : 'border-border-subtle bg-bg-subtle hover:border-border-default hover:bg-bg-overlay',
      )}
    >
      <span className={cn('text-sm font-bold tabular-nums', isSelected ? 'text-primary-400' : 'text-text-primary')}>
        {option.startTime}
      </span>
      {typeLabel && <span className="text-micro text-text-secondary mt-0.5">{typeLabel}</span>}
      {isLast && (
        <span className="text-micro text-warning-text mt-0.5 font-medium">{t.publicBooking.lastCourtLeft}</span>
      )}
    </button>
  );
}
