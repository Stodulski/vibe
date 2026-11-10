import { useEffect, useMemo, useRef } from 'react';
import { BOOKING_MESSAGES } from '../messages';
import type { CourtAvailability } from '../api/types';
import { TimeGroupList } from './court-selector/TimeGroupList';
import { CourtQuestion } from './court-selector/CourtQuestion';
import { useCourtSelectorState } from './court-selector/useCourtSelectorState';
import { buildTimeOptions, groupTimeOptions, type CourtAtTime, type TimeOption } from './court-selector/timeOptions';
import type { SelectedSlot } from './court-selector/types';

export type { SelectedSlot } from './court-selector/types';

const t = BOOKING_MESSAGES;

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
 * afterwards, always, even when only one is free: the player sees which court
 * and what it costs before the confirm page, and that tap is the one that
 * continues, so there is no separate Continue button.
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
  // "Última cancha" only means something where there was more than one.
  const severalCourts = courts.length > 1;
  // `pendingStartTime` already answers itself away once its hour stops being
  // on offer — the page derives it from the same availability this grid is
  // built from (see `ComplexPage`'s `isStartTimeStillOffered`) rather than
  // this component writing it back up. So a `pendingStartTime` this grid does
  // not recognize is simply not found here either, with no effect needed to
  // reconcile the two.
  // An hour restored from the confirm page's "Cambiar horario" link
  // (`?time=`) reopens its court question directly.
  const pendingTime = options.find((o) => o.startTime === pendingStartTime) ?? null;

  // An hour that was already chosen when the grid first appeared came from
  // the URL: "Cambiar horario" on the confirm page, or the browser's back.
  // The page would otherwise open at the top, a header and a date strip away
  // from the question it reopened, so it is brought into view once. An hour
  // tapped on this page is already where the person is looking.
  const questionRef = useRef<HTMLDivElement>(null);
  const restoredOnMount = useRef(pendingTime !== null);
  useEffect(() => {
    if (!restoredOnMount.current) return;
    // Cleared inside the frame, not before it: under StrictMode the effect
    // runs twice and the first frame is cancelled, so the second run must
    // still find the flag set.
    const frame = requestAnimationFrame(() => {
      restoredOnMount.current = false;
      questionRef.current?.scrollIntoView({ block: 'start' });
    });
    return () => {
      cancelAnimationFrame(frame);
    };
  }, []);

  function handleSelectTime(option: TimeOption) {
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
    return (
      <div ref={questionRef} className="scroll-mt-6">
        <CourtQuestion option={pendingTime} selected={selected} onSelectCourt={handleSelectCourt} />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <TimeGroupList
        grouped={grouped}
        typesVary={typesVary}
        severalCourts={severalCourts}
        selected={selected}
        onSelectTime={handleSelectTime}
      />
    </div>
  );
}
