import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import type { CourtAvailability } from '../../api/types';
import { TimeRuler } from './TimeRuler';
import { buildTimeOptions } from './timeOptions';

function day(times: string[]): CourtAvailability[] {
  return [
    {
      court_id: 'c1',
      court_name: 'Pádel 1',
      sport: 'padel',
      court_type: 'outdoor',
      has_lighting: true,
      slots: times.map((time) => {
        const [h = '0', m = '0'] = time.split(':');
        return { start_time: time, end_time: '', start_min: Number(h) * 60 + Number(m), duration_minutes: 90, price: 1000, available: true };
      }),
    },
  ];
}

const options = buildTimeOptions(day(['08:00', '09:00', '10:00', '11:00', '12:00']));

describe('TimeRuler', () => {
  it('opens on the middle hour of the day when nothing is chosen', () => {
    render(<TimeRuler options={options} initialStartTime={null} severalCourts={false} onSelectTime={vi.fn()} />);

    expect(screen.getByRole('button', { name: '10:00' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('opens on the hour already chosen', () => {
    render(<TimeRuler options={options} initialStartTime="08:00" severalCourts={false} onSelectTime={vi.fn()} />);

    expect(screen.getByRole('button', { name: '08:00' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('is not endless: each hour appears once, and none before the first', () => {
    render(<TimeRuler options={options} initialStartTime="08:00" severalCourts={false} onSelectTime={vi.fn()} />);

    expect(screen.getAllByRole('button', { name: '12:00' })).toHaveLength(1);
    expect(screen.getAllByRole('button', { name: /^\d\d:\d\d$/ }).map((tick) => tick.getAttribute('aria-label'))).toEqual([
      '08:00',
      '09:00',
      '10:00',
      '11:00',
      '12:00',
    ]);
  });
});
