import { BOOKING_MESSAGES } from '../../messages';
import type { PublicOpenPlayEvent } from '../../api/types';

const t = BOOKING_MESSAGES.events;
const SPORT_LABELS: Record<string, string> = BOOKING_MESSAGES.courts.sportTypes;

export function sportLabel(sport: string): string {
  return SPORT_LABELS[sport] ?? sport;
}

/** "19:00 a 22:00 hs". The end is read as the next day by the server when it is not later than the start. */
export function eventTimeRange(event: Pick<PublicOpenPlayEvent, 'start_time' | 'end_time'>): string {
  return `${event.start_time} ${t.toTime} ${event.end_time} ${t.hoursSuffix}`;
}

export function endsNextDay(event: Pick<PublicOpenPlayEvent, 'start_time' | 'end_time'>): boolean {
  return event.end_time <= event.start_time;
}

export function spotsLeftLabel(spotsLeft: number): string {
  if (spotsLeft === 1) return t.spotsLeftOne;
  return `${t.spotsLeftPrefix} ${String(spotsLeft)} ${t.spotsLeftSuffix}`;
}

/** Whether a visitor can still take a spot. The server decides for real; this only shapes the page. */
export function isOpenForSignUp(event: Pick<PublicOpenPlayEvent, 'status' | 'spots_left'>): boolean {
  return event.status === 'active' && event.spots_left > 0;
}
