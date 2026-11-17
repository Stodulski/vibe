import { formatPrice } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { courtFactsLine } from '../../lib/courtFacts';
import { keepTogether } from '../../lib/keepTogether';
import { ChoiceWheel } from '../choice-wheel/ChoiceWheel';
import { SportGlyph } from '../choice-wheel/SportGlyph';
import type { CourtAtTime, TimeOption } from './timeOptions';

const t = BOOKING_MESSAGES;

interface CourtPickerProps {
  option: TimeOption;
  selectedCourtId: string | null;
  onSelect: (entry: CourtAtTime) => void;
}

/**
 * The courts free at the chosen hour, on the same wheel as the sport: each one drawn
 * from above, with its facts and its price, the one at the center large and green.
 * It opens on the first court; tapping the centered one, or "Reservar <cancha>", books it.
 */
export function CourtPicker({ option, selectedCourtId, onSelect }: CourtPickerProps) {
  const byId = new Map(option.courts.map((entry) => [entry.court.court_id, entry]));
  return (
    <ChoiceWheel
      question={t.publicBooking.selectCourt}
      startAt="first"
      size="large"
      selected={selectedCourtId}
      choices={option.courts.map(({ court, slot }) => ({
        value: court.court_id,
        label: keepTogether(court.court_name),
        title: court.court_name,
        art: <SportGlyph sport={court.sport} />,
        detail: (
          <>
            <span className="text-text-tertiary w-full truncate text-xs">
              {(t.courts.sportTypes as Record<string, string | undefined>)[court.sport] ?? court.sport} · {courtFactsLine(court)}
            </span>
            <span className="text-base font-semibold tabular-nums">{formatPrice(slot.price)}</span>
            <span className={court.description ? 'text-text-secondary w-full truncate text-xs' : 'text-text-tertiary w-full truncate text-xs italic'}>
              {court.description ?? t.courts.noDescription}
            </span>
          </>
        ),
      }))}
      continueLabel={(choice) => `${t.publicBooking.reserveCourt} ${choice.label}`}
      onChoose={(courtId) => {
        const entry = byId.get(courtId);
        if (entry) onSelect(entry);
      }}
    />
  );
}
