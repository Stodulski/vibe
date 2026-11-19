import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import type { Schedule } from '../../api/types';
import { DateButton } from './DateButton';

// Open every day, so the tests are about the labels and not the closed state.
const openEveryDay: Schedule[] = (
  ['sunday', 'monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday'] as const
).map((day, i) => ({
  id: `s${String(i)}`,
  complex_id: 'c1',
  day,
  open_time: '08:00',
  close_time: '23:00',
  is_closed: false,
}));

const today = new Date(2026, 9, 12);
// A Wednesday, so the weekday label reads "mié" and the month reads "oct".
const day = new Date(2026, 9, 14);

function renderButton(selected: boolean) {
  return render(
    <DateButton
      date={day}
      today={today}
      selectedDate={selected ? day : today}
      schedules={openEveryDay}
      onDateSelect={vi.fn()}
    />,
  );
}

describe('DateButton labels', () => {
  it('reads the weekday and the month of the selected day', () => {
    renderButton(true);

    expect(screen.getByText('mié')).toBeInTheDocument();
    expect(screen.getByText('oct')).toBeInTheDocument();
  });
});
