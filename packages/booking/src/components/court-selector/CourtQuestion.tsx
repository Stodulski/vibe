import { CourtPicker } from './CourtPicker';
import type { CourtAtTime, TimeOption } from './timeOptions';
import type { SelectedSlot } from './types';

interface CourtQuestionProps {
  option: TimeOption;
  selected: SelectedSlot | null;
  /** Choosing a court answers the last question and continues. */
  onSelectCourt: (entry: CourtAtTime) => void;
}

/**
 * The court question, owning the screen while it is open.
 *
 * The hours are gone while it is asked; the chosen one is shown by the page
 * as a crumb beside the sport and duration, and that crumb is the way back.
 * Both hours and courts on screen at once let a visitor tap a second hour
 * while a court from the first was still highlighted, and the panel then
 * named one hour while the grid marked another.
 *
 * No Continue button: like the sport and the duration, the answer is the tap.
 */
export function CourtQuestion({ option, selected, onSelectCourt }: CourtQuestionProps) {
  // The selection counts here only if it was made at this very hour. A court
  // is highlighted by id, and the same court free at two hours would
  // otherwise show as chosen at both.
  const settledHere = selected?.slot.start_time === option.startTime;

  return (
    <CourtPicker option={option} selectedCourtId={settledHere ? selected.courtId : null} onSelect={onSelectCourt} />
  );
}
