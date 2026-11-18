import { formatPrice } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { courtFactParts } from '../../lib/courtFacts';
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
        label: court.court_name,
        title: court.court_name,
        art: <SportGlyph sport={court.sport} />,
        detail: (
          <>
            {/* One fact a line: roof, surface, lighting. The sport is the drawing above. */}
            <ul className="text-text-secondary w-full space-y-0.5 text-xs">
              {courtFactParts(court).map((fact) => (
                <li key={fact}>{fact}</li>
              ))}
            </ul>
            <span className="mt-auto text-base font-semibold tabular-nums">{formatPrice(slot.price)}</span>
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
