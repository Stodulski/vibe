import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { BookingBlock } from './BookingBlock';
import { GRID_HEIGHT_PX } from './gridLayout';
import { makeBooking } from '@/test/factories';
import { VENUE_TIME_ZONE, venueInstant } from '@/shared/lib/instants';
import { timeToMinutes } from '@/shared/lib/time';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

const DAY = '2026-08-28';

/**
 * The instant a wall-clock reading names, in the venue's own zone
 * (`VENUE_TIME_ZONE`) — never the test process's. `BookingBlock`'s
 * positioning math (`minutesInto`/`startOfDay` in `shared/lib/instants.ts`)
 * and `formatHourRange` both always read a calendar day and format an
 * instant in the venue zone, so a fixture built from the process's own zone
 * would name a different real moment — and land in the wrong place on the
 * grid or show the wrong hour — on any machine or CI runner not itself set
 * to Argentina time.
 */
function atVenue(date: string, hhmm: string): string {
  const instant = venueInstant(date, timeToMinutes(hhmm));
  if (!instant) throw new Error(`invalid fixture date/time: ${date} ${hhmm}`);
  return instant;
}

/** Reads the `HH:MM` a venue-zone clock would show for an instant. */
const venueClock = new Intl.DateTimeFormat('en-GB', {
  timeZone: VENUE_TIME_ZONE,
  hour: '2-digit',
  minute: '2-digit',
  hourCycle: 'h23',
});

/**
 * The time-of-day fields are kept consistent with the instants on purpose. A
 * fixture where they disagree would let a test pass for the wrong reason, and
 * the cross-midnight cases below are already a clean discriminator: no pair of
 * clock readings on one date can describe a span that leaves it.
 *
 * `viewDay` is the calendar day the grid displays. It defaults to the day the
 * booking starts on, which is always what the booking record itself carries;
 * a caller viewing a cross-midnight booking from the day it *ends* on passes
 * that day here without moving the booking's own `date`.
 */
function renderBlock(startsAt: string, endsAt: string, viewDay = DAY) {
  const hhmm = (iso: string) => venueClock.format(new Date(iso));
  const booking = makeBooking({
    starts_at: startsAt,
    ends_at: endsAt,
    date: DAY,
    start_time: hhmm(startsAt),
    duration_minutes: Math.round((new Date(endsAt).getTime() - new Date(startsAt).getTime()) / 60_000),
  });
  render(<BookingBlock booking={booking} courtName="Cancha 1" date={viewDay} onSelect={() => undefined} />);
  return screen.getByRole('button');
}

/**
 * The percentage out of `calc(<n>% ± 1px)`. The component insets each block by
 * a pixel per side so two back-to-back bookings do not draw a doubled seam;
 * that pixel is presentation, and these tests are about the percentage.
 */
const pct = (v: string | null) => Number(/(-?[\d.]+)%/.exec(v ?? '')?.[1] ?? NaN);

describe('BookingBlock semantics', () => {
  // It used to be a div with role="button", tabIndex and a hand-written
  // Enter/Space handler. A real button gets all of that from the platform,
  // including the keyboard activation asserted here (A11Y-02).
  it('is a real button that activates from the keyboard', async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    const booking = makeBooking({ date: DAY, start_time: '10:00', duration_minutes: 60 });
    render(<BookingBlock booking={booking} courtName="Cancha 1" date={DAY} onSelect={onSelect} />);

    const el = screen.getByRole('button');
    expect(el.tagName).toBe('BUTTON');
    expect(el).toHaveAttribute('type', 'button');

    el.focus();
    await user.keyboard('{Enter}');
    await user.keyboard(' ');

    expect(onSelect).toHaveBeenCalledTimes(2);
  });
});

describe('BookingBlock placement', () => {
  it('places a booking inside the day at its own hours', () => {
    const el = renderBlock(atVenue(DAY, '10:00'), atVenue(DAY, '11:30'));

    expect(pct(el.style.top)).toBeCloseTo((600 / 1440) * 100, 3);
    expect(pct(el.style.height)).toBeCloseTo((90 / 1440) * 100, 3);
  });

  // The bug this component was rewritten for. Read from the pair of clock
  // readings, a booking running 23:00 to 01:00 gave endMin (60) below startMin (1380);
  // the guard for that treated it as corrupt data and drew a 30-minute bar at
  // 23:00 — two hours of court shown as half an hour, with no error anywhere.
  it('clips a booking that runs into tomorrow at the bottom of today', () => {
    const el = renderBlock(atVenue(DAY, '23:00'), atVenue('2026-08-29', '01:00'));

    expect(pct(el.style.top)).toBeCloseTo((1380 / 1440) * 100, 3);
    // Only the hour before midnight belongs to this day.
    expect(pct(el.style.height)).toBeCloseTo((60 / 1440) * 100, 3);
  });

  it('clips a booking that began yesterday at the top of today', () => {
    const el = renderBlock(atVenue('2026-08-27', '23:00'), atVenue(DAY, '01:00'));

    expect(pct(el.style.top)).toBe(0);
    // And only the hour after midnight belongs to this one.
    expect(pct(el.style.height)).toBeCloseTo((60 / 1440) * 100, 3);
  });

  it('keeps the height and the position describing the same stretch', () => {
    const el = renderBlock(atVenue('2026-08-27', '23:00'), atVenue(DAY, '01:00'));

    const heightPx = (pct(el.style.height) / 100) * GRID_HEIGHT_PX;
    const topPx = (pct(el.style.top) / 100) * GRID_HEIGHT_PX;
    expect(topPx + heightPx).toBeLessThanOrEqual(GRID_HEIGHT_PX);
  });
});

// A sibling describe, not nested in the one above: max-lines-per-function
// counts a describe callback's whole body, so this stays a genuinely
// separate top-level call rather than a subgroup.
describe('BookingBlock time display and instant fallback', () => {
  // The exact case the grid lost. A 60-minute block is 56px, which is the
  // threshold for fitting two lines — and deriving those pixels from the
  // clipped percentage produced 55.999999999999986, so every one-hour booking
  // on the grid silently dropped its time and showed the name alone.
  it('shows the time on an hour-long booking, which sits exactly on the two-line threshold', () => {
    const el = renderBlock(atVenue(DAY, '10:00'), atVenue(DAY, '11:00'));
    expect(el.textContent).toContain('10:00');
    expect(el.textContent).toContain('11:00');
  });

  it('shows the time on the visible hour of a booking clipped at the top', () => {
    const el = renderBlock(atVenue('2026-08-27', '23:00'), atVenue(DAY, '01:00'));
    expect(el.textContent).toContain('23:00');
  });

  // Below the threshold there is genuinely room for one line, and the name is
  // the one worth keeping — the hour is already legible from where the block
  // sits on the grid.
  it('drops the time when the block is too short for two lines', () => {
    const el = renderBlock(atVenue(DAY, '10:00'), atVenue(DAY, '10:30'));
    expect(el.textContent).not.toContain('10:30');
    expect(el.textContent).toContain('Juan Garcia');
  });

  it('falls back to the start plus the duration when the instants are missing', () => {
    // A response cached before the server began sending them.
    const booking = makeBooking({
      starts_at: '',
      ends_at: '',
      date: DAY,
      start_time: '10:00',
      duration_minutes: 90,
    });
    render(<BookingBlock booking={booking} courtName="Cancha 1" date={DAY} onSelect={() => undefined} />);

    const el = screen.getByRole('button');
    expect(pct(el.style.top)).toBeCloseTo((600 / 1440) * 100, 3);
    expect(pct(el.style.height)).toBeCloseTo((90 / 1440) * 100, 3);
  });

  // The fallback used to read the stored end, which wrapped: a 23:00 booking of
  // two hours came back as "01:00" and the block was drawn thirty minutes long.
  // Adding the duration cannot wrap, so the overnight case is drawn right even
  // without the instants.
  it('draws an overnight booking from the duration when the instants are missing', () => {
    const booking = makeBooking({
      starts_at: '',
      ends_at: '',
      date: DAY,
      start_time: '23:00',
      duration_minutes: 120,
    });
    render(<BookingBlock booking={booking} courtName="Cancha 1" date={DAY} onSelect={() => undefined} />);

    const el = screen.getByRole('button');
    expect(pct(el.style.top)).toBeCloseTo((1380 / 1440) * 100, 3);
    expect(pct(el.style.height)).toBeCloseTo((60 / 1440) * 100, 3);
  });
});

describe('BookingBlock hours', () => {
  // The visible range carries no "Día sig." marker any more (removed
  // dialog-wide, and range-wide, by owner instruction) — `formatHourRange`
  // itself no longer prints one. The fact still has to reach a screen reader
  // somehow, so the block's own accessible name states it in full instead
  // (`t.bookings.endsNextDay`), read straight from `endsOnALaterDay` rather
  // than re-derived, so the visible block and the announced one can never
  // disagree about the same booking.
  //
  // Viewed from the day the booking ends on rather than the day it starts:
  // whether the accessible name says so never depends on which day's grid
  // renders it, but the visible height backing `showBoth` does, and an
  // evening-to-early-morning booking is only guaranteed a visible sliver on
  // the *later* day once its instants are read in the venue zone on a test
  // process running in some other one — the exact case `atVenue` exists for.
  it('marks a booking whose hours end on the following day, in its accessible name only', () => {
    const el = renderBlock(atVenue(DAY, '23:00'), atVenue('2026-08-29', '01:00'), '2026-08-29');

    expect(el.textContent).not.toContain('Día sig.');
    expect(el.getAttribute('aria-label')).not.toContain('Día sig.');
    expect(el.getAttribute('aria-label')).toContain(t.bookings.endsNextDay);
  });

  // The control: an ordinary booking carries no marker anywhere, so the
  // assertion above is not passing on a helper that appends the note to
  // everything.
  it('leaves an ordinary booking unmarked', () => {
    const el = renderBlock(atVenue(DAY, '10:00'), atVenue(DAY, '11:30'));

    expect(el.textContent).not.toContain('Día sig.');
    expect(el.getAttribute('aria-label')).not.toContain(t.bookings.endsNextDay);
    expect(el.textContent).toContain('10:00');
    expect(el.textContent).toContain('11:30');
  });
});
