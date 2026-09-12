import { useMemo } from 'react';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { CourtAvailability } from '@/shared/types/api.types';
import { TimeGroupList } from './court-selector/TimeGroupList';
import { CourtQuestion } from './court-selector/CourtQuestion';
import { useCourtSelectorState } from './court-selector/useCourtSelectorState';
import { buildTimeOptions, groupTimeOptions, type CourtAtTime, type TimeOption } from './court-selector/timeOptions';
import { needsCourtChoice } from './court-selector/courtChoice';
import type { SelectedSlot } from './court-selector/types';

export type { SelectedSlot } from './court-selector/types';

const t = ES_AR;

interface CourtSelectorProps {
  courts: CourtAvailability[];
  selected: SelectedSlot | null;
  onSelect: (selection: SelectedSlot) => void;
  /**
   * Off to confirm. Called with the selection when the tap that chose a court
   * is also the tap that continues, so the page does not have to read a
   * selection it has not stored yet.
   */
  onContinue: (selection?: SelectedSlot) => void;
  /**
   * The hour whose court question is open, held by the page rather than here
   * so the page can show it as a crumb next to the sport and duration. Held
   * as a time rather than as the option object: availability refetches on its
   * own — a duration change, a new date, someone else booking — and a stored
   * object would keep rendering the courts that were free a minute ago.
   */
  pendingStartTime: string | null;
  onPendingStartTimeChange: (startTime: string | null) => void;
}

/**
 * Availability, asked in the order people actually decide.
 *
 * It used to be court-major: a card per court, each with its own duration
 * picker and its own copy of every hour. Twelve courts turned forty-five
 * distinct times into hundreds of buttons and drew the duration control a
 * dozen times, and nobody arrives wanting court 7 — they arrive wanting
 * Saturday at 20:00. The hour is the question now; the court is asked
 * afterwards, and only when the free courts differ in some way worth choosing
 * between.
 *
 * The duration sits at the top, once. It filters the entire search — the
 * server generates the grid of valid start times from it — so it never
 * belonged inside a card describing one court.
 */
export function CourtSelector({
  courts,
  selected,
  onSelect,
  onContinue,
  pendingStartTime,
  onPendingStartTimeChange,
}: CourtSelectorProps) {
  const { handleSlotClick } = useCourtSelectorState({ onSelect });

  const options = useMemo(() => buildTimeOptions(courts), [courts]);
  const grouped = useMemo(() => groupTimeOptions(options), [options]);
  // Whether this venue has courts of more than one type — decided once for
  // the whole grid. A club whose courts are all covered has nothing to warn
  // about, and a label that never varies is not a label.
  const typesVary = new Set(courts.map((court) => court.court_type)).size > 1;
  // `pendingStartTime` already answers itself away once its hour stops being
  // on offer — the page derives it from the same availability this grid is
  // built from (see `ComplexPage`'s `isStartTimeStillOffered`) rather than
  // this component writing it back up. So a `pendingStartTime` this grid does
  // not recognize is simply not found here either, with no effect needed to
  // reconcile the two.
  const matchedTime = options.find((o) => o.startTime === pendingStartTime) ?? null;
  // Only a hint worth opening the court question over. `handleSelectTime`
  // below only ever writes `pendingStartTime` when `needsCourtChoice` is
  // true — but this prop can also arrive already set on mount, from the
  // confirm page's "Cambiar horario" link, which round-trips the previously
  // chosen hour through `?time=` regardless of whether that hour ever needed
  // a court question (U-06). `CourtPicker` renders nothing when there is
  // nothing to choose between, so without this check a returning visitor who
  // had a single-court hour landed on a blank panel: the crumb said
  // "Horario 16:00", and below it, nothing — no hours, no courts, no button.
  // Falling through to the grid instead is "reopening the hour step": the
  // visitor sees every hour again, including the one they had, rather than
  // a dead end they can only escape through the crumb's pencil.
  const pendingTime = matchedTime && needsCourtChoice(matchedTime.courts) ? matchedTime : null;

  function handleSelectTime(option: TimeOption) {
    const first = option.courts[0];
    // Nothing to choose between: assign and skip the question entirely.
    if (!needsCourtChoice(option.courts) && first) {
      handleSlotClick(first.court, first.slot);
      onPendingStartTimeChange(null);
      return;
    }
    onPendingStartTimeChange(option.startTime);
  }

  // Choosing a court is the last question, so it continues on its own, the
  // way answering the sport or the duration moves on without a button.
  function handleSelectCourt(entry: CourtAtTime) {
    const selection = handleSlotClick(entry.court, entry.slot);
    if (selection) onContinue(selection);
  }

  if (options.length === 0) {
    return <p className="text-text-tertiary py-8 text-center text-sm">{t.publicBooking.noAvailability}</p>;
  }

  // Either the hours or the courts, never both — see `CourtQuestion`.
  if (pendingTime) {
    return <CourtQuestion option={pendingTime} selected={selected} onSelectCourt={handleSelectCourt} />;
  }

  return (
    <div className="space-y-6">
      <TimeGroupList
        grouped={grouped}
        typesVary={typesVary}
        selected={selected}
        onSelectTime={handleSelectTime}
        onContinue={onContinue}
      />
    </div>
  );
}
