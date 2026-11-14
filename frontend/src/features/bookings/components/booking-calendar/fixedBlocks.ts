import { spanOnDay, type Span } from '@/shared/lib/instants';
import { timeToMinutes } from '@/shared/lib/time';
import { DAY_END_MIN, DAY_START_MIN } from './gridLayout';
import type { FixedBlock } from '@/shared/types/api.types';

/**
 * The hours a fixed weekly booking or an open-play event holds on a court:
 * both arrive with the same date and clock readings, so both are measured and
 * treated as obstacles the same way.
 */
export type HeldBlock = Pick<FixedBlock, 'court_id' | 'date' | 'start_time' | 'end_time'>;

/** The minutes of a grid day a held block covers, from midnight of that day. */
export interface HeldRange {
  startMin: number;
  endMin: number;
  /** True when the block started on an earlier day and this is only its tail. */
  isTail: boolean;
  /** True when the block runs on past this day's end, into the next one. */
  runsOn: boolean;
}

/**
 * Where a held block sits on the grid of `date`. The API sends clock readings
 * with the date the block starts on, so hours that run past midnight arrive
 * with an `end_time` earlier than the start. On the start date they are drawn
 * to the end of the day; on the next day the grid draws the tail, from 00:00 to
 * the end. A block of any other date is not on this day.
 */
export function heldBlockRange(block: HeldBlock, date: string): HeldRange | null {
  const startMin = timeToMinutes(block.start_time);
  const endMin = timeToMinutes(block.end_time);
  const overnight = endMin <= startMin;
  const blockDate = block.date.slice(0, 10);
  if (blockDate === date.slice(0, 10)) {
    return { startMin, endMin: overnight ? DAY_END_MIN : endMin, isTail: false, runsOn: overnight };
  }
  if (overnight && nextDay(blockDate) === date.slice(0, 10)) {
    return { startMin: DAY_START_MIN, endMin, isTail: true, runsOn: false };
  }
  return null;
}

function nextDay(date: string): string {
  const day = new Date(`${date}T00:00:00Z`);
  day.setUTCDate(day.getUTCDate() + 1);
  return day.toISOString().slice(0, 10);
}

/** The spans the court's held blocks hold on `date`, to be treated like any other obstacle for free slots. */
export function fixedBlockSpans(blocks: HeldBlock[], courtId: string, date: string): Span[] {
  const spans: Span[] = [];
  for (const block of blocks) {
    if (block.court_id !== courtId) continue;
    const range = heldBlockRange(block, date);
    if (!range) continue;
    const span = spanOnDay(date, range.startMin, range.endMin);
    if (span) spans.push(span);
  }
  return spans;
}
