import { describe, it, expect } from 'vitest';
import type { CourtAvailability } from '../../api/types';
import { readWaitlistNotice, waitlistNoticeApplies } from './waitlistNotice';

const COURT = '4f8b2c1e-9a3d-4e5f-8b7c-6d1e2f3a4b5c';

function court(id: string, available: boolean): CourtAvailability {
  return {
    court_id: id,
    court_name: 'Pádel 1',
    sport: 'padel',
    court_type: 'indoor',
    has_lighting: true,
    slots: [{ start_time: '19:00', end_time: '20:30', start_min: 1140, duration_minutes: 90, price: 4200000, available }],
  };
}

const link = new URLSearchParams(`court=${COURT}&date=2026-11-19&duration=90&sport=padel&time=19:00`);
const notice = readWaitlistNotice(link);
const atTheHour = { date: '2026-11-19', time: '19:00', duration: 90 };

describe('the notice for someone who arrives from a waitlist email', () => {
  it('reads the court, day, hour and duration the email was about', () => {
    expect(notice).toEqual({ courtId: COURT, date: '2026-11-19', time: '19:00', duration: 90 });
  });

  it('is not read from a link without a court, or with a malformed one', () => {
    expect(readWaitlistNotice(new URLSearchParams('date=2026-11-19&duration=90&time=19:00'))).toBeNull();
    expect(readWaitlistNotice(new URLSearchParams('court=x&date=2026-11-19&duration=90&time=19:00'))).toBeNull();
    expect(readWaitlistNotice(new URLSearchParams(`court=${COURT}&date=2026-11-19&duration=90`))).toBeNull();
  });

  it('applies when that court is no longer free at that hour', () => {
    expect(waitlistNoticeApplies(notice, atTheHour, [court(COURT, false), court('other', true)])).toBe(true);
  });

  it('applies when that court is gone from the grid at that hour', () => {
    expect(waitlistNoticeApplies(notice, atTheHour, [court('other', true)])).toBe(true);
  });

  it('does not apply while that court is still free: the person can book it', () => {
    expect(waitlistNoticeApplies(notice, atTheHour, [court(COURT, true)])).toBe(false);
  });

  it('does not apply once the person looks at another day, hour or duration', () => {
    const taken = [court(COURT, false)];
    expect(waitlistNoticeApplies(notice, { ...atTheHour, date: '2026-11-20' }, taken)).toBe(false);
    expect(waitlistNoticeApplies(notice, { ...atTheHour, time: '20:30' }, taken)).toBe(false);
    expect(waitlistNoticeApplies(notice, { ...atTheHour, duration: 60 }, taken)).toBe(false);
  });

  it('does not apply without a notice', () => {
    expect(waitlistNoticeApplies(null, atTheHour, [court(COURT, false)])).toBe(false);
  });
});
