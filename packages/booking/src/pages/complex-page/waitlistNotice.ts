import type { CourtAvailability } from '../../api/types';

/**
 * The place a waitlist email told someone had freed: the court, the day, the
 * hour and the duration its link opens the page on (`booklink.StorefrontSlot`).
 * Several people get the same email at once and the first to pay keeps the
 * place, so whoever arrives second may find it taken.
 */
export interface WaitlistNotice {
  courtId: string;
  date: string;
  time: string;
  duration: number;
}

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const DATE = /^\d{4}-\d{2}-\d{2}$/;
const TIME = /^([01]\d|2[0-3]):[0-5]\d$/;

/** The notice a waitlist link carries, or `null` when the link is not one. */
export function readWaitlistNotice(params: URLSearchParams): WaitlistNotice | null {
  const courtId = params.get('court') ?? '';
  const date = params.get('date') ?? '';
  const time = params.get('time') ?? '';
  const duration = Number(params.get('duration'));
  if (!UUID.test(courtId) || !DATE.test(date) || !TIME.test(time) || !Number.isInteger(duration) || duration <= 0) {
    return null;
  }
  return { courtId, date, time, duration };
}

/**
 * Whether to tell the visitor someone took the place first: they are looking
 * at the very day, hour and duration the email was about, and that court is no
 * longer free then. Once they move to another hour the notice is no longer
 * about what they see.
 */
export function waitlistNoticeApplies(
  notice: WaitlistNotice | null,
  view: { date: string; time: string | null; duration: number },
  courts: CourtAvailability[],
): boolean {
  if (!notice) return false;
  if (view.date !== notice.date || view.time !== notice.time || view.duration !== notice.duration) return false;
  const slot = courts
    .find((c) => c.court_id === notice.courtId)
    ?.slots.find((s) => s.start_time === notice.time);
  return !slot?.available;
}
