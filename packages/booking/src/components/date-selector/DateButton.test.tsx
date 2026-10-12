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
  // The selected fill is the primary colour at 20% over the page. The tertiary grey measures
  // 4.1-4.3:1 on it, below AA; the secondary grey clears 4.5:1 with room to spare.
  it('reads the weekday and the month of the selected day in the secondary grey', () => {
    renderButton(true);

    expect(screen.getByText('mié')).toHaveClass('text-text-secondary');
    expect(screen.getByText('oct')).toHaveClass('text-text-secondary');
    expect(screen.getByText('mié')).not.toHaveClass('text-text-tertiary');
  });

  it('keeps the tertiary grey on the labels of an unselected day', () => {
    renderButton(false);

    expect(screen.getByText('mié')).toHaveClass('text-text-tertiary');
    expect(screen.getByText('oct')).toHaveClass('text-text-tertiary');
  });
});
