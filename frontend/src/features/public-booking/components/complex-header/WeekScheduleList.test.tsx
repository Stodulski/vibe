import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { WeekScheduleList } from './WeekScheduleList';
import type { Schedule } from '@/shared/types/api.types';

function day(name: string, open: string, close: string): Schedule {
  return {
    id: name,
    complex_id: 'c1',
    day: name,
    open_time: open,
    close_time: close,
    is_closed: false,
  } as Schedule;
}

const EVERY_DAY = ['monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday'];
const schedules = EVERY_DAY.map((d) => day(d, '08:00', '23:00'));

describe('WeekScheduleList', () => {
  // A Saturday, chosen because it is never "today" in CI.
  const aSaturday = new Date(2026, 2, 21);

  it('bolds the day carried by selectedDate, not today', () => {
    render(<WeekScheduleList schedules={schedules} selectedDate={aSaturday} />);

    expect(screen.getByText('Sábado')).toHaveClass('font-semibold', 'text-primary-400');
  });

  it('does not bold any other day when selectedDate is given', () => {
    render(<WeekScheduleList schedules={schedules} selectedDate={aSaturday} />);

    expect(screen.getByText('Lunes')).not.toHaveClass('text-primary-400');
  });

  it('falls back to bolding today when no selectedDate is given', () => {
    render(<WeekScheduleList schedules={schedules} />);

    // Whatever CI's actual today is, exactly one label carries the "today" styling.
    const dayLabels = screen.getAllByText(/^(Lunes|Martes|Miércoles|Jueves|Viernes|Sábado|Domingo)$/);
    const highlighted = dayLabels.filter((el) => el.className.includes('text-primary-400'));
    expect(highlighted).toHaveLength(1);
  });
});
