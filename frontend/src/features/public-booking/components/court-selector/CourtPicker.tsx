import { cn, formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { COURT_TYPE_LABELS } from './constants';
import { needsCourtChoice } from './courtChoice';
import type { CourtAtTime, TimeOption } from './timeOptions';

const t = ES_AR;

const SPORT_LABELS: Record<string, string> = t.courts.sportTypes;

interface CourtCardProps {
  entry: CourtAtTime;
  isSelected: boolean;
  onSelect: (entry: CourtAtTime) => void;
}

/**
 * One court as a card, read top to bottom: name, sport and type, the price
 * at this hour, then the description. A court without a description says so
 * instead of being shorter than its neighbours, so every card keeps the same
 * shape.
 */
function CourtCard({ entry, isSelected, onSelect }: CourtCardProps) {
  const { court, slot } = entry;
  const emphasis = isSelected ? 'text-primary-400' : 'text-text-primary';

  return (
    <button
      onClick={() => {
        onSelect(entry);
      }}
      aria-pressed={isSelected}
      className={cn(
        // Visible idle edge + a slight fill + a stronger selected fill than
        // idle (odd/tasks/app-dark-contrast.md T4) — see `DateButton` for the
        // same change. Idle used to be `bg-bg-base`, the page's own color, so
        // its "fill" had zero contrast against the page it sits on; it is now
        // the same card-tier fill the date and slot buttons use.
        'flex cursor-pointer flex-col items-start gap-1 rounded-lg border p-3 text-left transition-colors duration-200',
        isSelected
          ? 'border-primary-500 bg-primary-500/20'
          : 'border-border-interactive bg-bg-subtle hover:border-border-interactive-hover hover:bg-bg-highlight',
      )}
    >
      {/* One line, cut with an ellipsis: a long name must not stretch its
          whole grid row. `title` keeps the full name a hover away. */}
      <span className={cn('w-full truncate text-sm font-semibold', emphasis)} title={court.court_name}>
        {court.court_name}
      </span>
      <span className="text-text-tertiary text-xs">
        {SPORT_LABELS[court.sport] ?? court.sport} · {COURT_TYPE_LABELS[court.court_type] ?? court.court_type}
      </span>
      <span className={cn('text-base font-semibold tabular-nums', emphasis)}>{formatPrice(slot.price)}</span>
      <span className={cn('text-xs', court.description ? 'text-text-secondary' : 'text-text-tertiary italic')}>
        {court.description ?? t.courts.noDescription}
      </span>
    </button>
  );
}

interface CourtPickerProps {
  option: TimeOption;
  selectedCourtId: string | null;
  onSelect: (entry: CourtAtTime) => void;
}

/**
 * The second question, asked only when there is one: which of the courts free
 * at the chosen hour.
 *
 * One flat list of cards, in the order the API sent them. Each card carries
 * what a player compares before choosing: the court's type, its sport, the
 * owner's description, and the price at this hour. The courts used to be
 * grouped under a type-and-price heading as name-only chips; the heading did
 * the comparing for the player and hid the description entirely.
 *
 * Renders nothing when `needsCourtChoice` finds nothing to choose between —
 * the caller has already assigned a court in that case, so a panel here would
 * be an empty ceremony around a decision already made.
 */
export function CourtPicker({ option, selectedCourtId, onSelect }: CourtPickerProps) {
  if (!needsCourtChoice(option.courts)) return null;

  return (
    <div className="mt-3">
      {/* The hour is already on the answered chip above; repeating it here
          would say the same thing twice on one screen. */}
      <p className="text-text-secondary text-xs font-semibold">{t.publicBooking.selectCourt}</p>
      {/* One card per row on a phone, so the name and the sport line never
          fight for width; three across from sm, four on a desktop. */}
      <div className="mt-3 grid grid-cols-1 gap-2 sm:grid-cols-3 lg:grid-cols-4">
        {option.courts.map((entry) => (
          <CourtCard
            key={entry.court.court_id}
            entry={entry}
            isSelected={entry.court.court_id === selectedCourtId}
            onSelect={onSelect}
          />
        ))}
      </div>
    </div>
  );
}
