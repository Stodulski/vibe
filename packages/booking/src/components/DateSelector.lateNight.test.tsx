import { afterAll, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { DateSelector } from './DateSelector';
import type { Schedule } from '../api/types';
import { todayInVenueTz } from '../lib/venueDate';

vi.useFakeTimers({ toFake: ['Date'] });
afterAll(() => {
  vi.useRealTimers();
});
Element.prototype.scrollIntoView = vi.fn();

const lateWednesday: Schedule[] = ['monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday'].map(
  (day, i) => ({
    id: `s${String(i)}`,
    complex_id: 'c1',
    day: day as Schedule['day'],
    open_time: '20:00',
    close_time: '02:00',
    is_closed: false,
  }),
);

// At 00:10 a venue open 20:00-02:00 is still selling last night's 00:30 and
// 01:00. The strip used to start on the new calendar day, whose window opens at
// 20:00, so those hours were never listed though the booking route takes them.
describe('DateSelector after midnight', () => {
  it("starts on last night's grid day while its window is still open", async () => {
    vi.setSystemTime(new Date('2026-10-15T03:10:00Z')); // Thursday 00:10 in Argentina
    const onDateSelect = vi.fn();
    const today = todayInVenueTz();
    render(<DateSelector selectedDate={today} onDateSelect={onDateSelect} schedules={lateWednesday} />);

    const days = screen.getAllByRole('radio');
    expect(days[0]?.getAttribute('aria-current')).toBeNull();
    expect(days[1]?.getAttribute('aria-current')).toBe('date');

    const lastNight = days[0];
    if (!lastNight) throw new Error('the strip lists no day');
    await userEvent.setup().click(lastNight);
    const picked = onDateSelect.mock.calls[0]?.[0] as Date;
    expect([picked.getFullYear(), picked.getMonth(), picked.getDate()]).toEqual([2026, 9, 14]);
  });

  it('starts on today once that window has closed', () => {
    vi.setSystemTime(new Date('2026-10-15T05:30:00Z')); // Thursday 02:30 in Argentina
    render(<DateSelector selectedDate={todayInVenueTz()} onDateSelect={vi.fn()} schedules={lateWednesday} />);

    expect(screen.getAllByRole('radio')[0]?.getAttribute('aria-current')).toBe('date');
  });
});
