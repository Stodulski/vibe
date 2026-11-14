import { spanOnDay, type Span } from '@/shared/lib/instants';
import { timeToMinutes } from '@/shared/lib/time';
import { DAY_END_MIN } from './gridLayout';
import type { FixedBlock } from '@/shared/types/api.types';

/**
 * The hours a fixed weekly booking or an open-play event holds on a court:
 * both arrive with the same date and clock readings, so both are measured and
 * treated as obstacles the same way.
 */
export type HeldBlock = Pick<FixedBlock, 'court_id' | 'date' | 'start_time' | 'end_time'>;

/**
 * Where a fixed block ends, in minutes from midnight of its date. The API sends
 * clock readings, so hours that run past midnight arrive with an `end_time`
 * earlier than the start; they are drawn to the end of this day (the rest
 * belongs to the next day's grid).
 */
export function fixedBlockEndMin(block: HeldBlock): number {
  const startMin = timeToMinutes(block.start_time);
  const endMin = timeToMinutes(block.end_time);
  return endMin <= startMin ? DAY_END_MIN : endMin;
}

/** The spans the court's fixed blocks hold on `date`, to be treated like any other obstacle for free slots. */
export function fixedBlockSpans(blocks: HeldBlock[], courtId: string, date: string): Span[] {
  const spans: Span[] = [];
  for (const block of blocks) {
    if (block.court_id !== courtId) continue;
    const span = spanOnDay(date, timeToMinutes(block.start_time), fixedBlockEndMin(block));
    if (span) spans.push(span);
  }
  return spans;
}
