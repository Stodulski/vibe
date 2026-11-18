import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { CourtAvailability } from '../../api/types';
import { TimeRuler } from './TimeRuler';
import { buildRulerHours } from './rulerHours';

function day(times: string[], taken: string[] = []): CourtAvailability[] {
  return [
    {
      court_id: 'c1',
      court_name: 'Pádel 1',
      sport: 'padel',
      court_type: 'outdoor',
      has_lighting: true,
      slots: times.map((time) => {
        const [h = '0', m = '0'] = time.split(':');
        return { start_time: time, end_time: '', start_min: Number(h) * 60 + Number(m), duration_minutes: 90, price: 1000, available: !taken.includes(time) };
      }),
    },
  ];
}

const hours = buildRulerHours(day(['08:00', '09:00', '10:00', '11:00', '12:00']));

describe('TimeRuler', () => {
  it('opens on the middle hour of the day when nothing is chosen', () => {
    render(<TimeRuler hours={hours} initialStartTime={null} severalCourts={false} onSelectTime={vi.fn()} />);

    expect(screen.getByRole('button', { name: '10:00' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('opens on the hour already chosen', () => {
    render(<TimeRuler hours={hours} initialStartTime="08:00" severalCourts={false} onSelectTime={vi.fn()} />);

    expect(screen.getByRole('button', { name: '08:00' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('is not endless: each hour appears once, and none before the first', () => {
    render(<TimeRuler hours={hours} initialStartTime="08:00" severalCourts={false} onSelectTime={vi.fn()} />);

    expect(screen.getAllByRole('button', { name: '12:00' })).toHaveLength(1);
    expect(screen.getAllByRole('button', { name: /^\d\d:\d\d$/ }).map((tick) => tick.getAttribute('aria-label'))).toEqual([
      '08:00',
      '09:00',
      '10:00',
      '11:00',
      '12:00',
    ]);
  });

  it('tapping an hour puts it at the center without choosing it', async () => {
    const user = userEvent.setup();
    const onSelectTime = vi.fn();
    render(<TimeRuler hours={hours} initialStartTime="08:00" severalCourts={false} onSelectTime={onSelectTime} />);

    await user.click(screen.getByRole('button', { name: '09:00' }));

    expect(screen.getByRole('button', { name: '09:00' })).toHaveAttribute('aria-pressed', 'true');
    expect(onSelectTime).not.toHaveBeenCalled();
    expect(screen.getByRole('button', { name: 'Ver canchas a las 09:00' })).toBeInTheDocument();
  });

  it('shows a taken hour in place, marked as taken, and offers the nearest free hour', async () => {
    const user = userEvent.setup();
    const onSelectTime = vi.fn();
    const busy = buildRulerHours(day(['08:00', '09:00', '10:00', '11:00', '12:00'], ['10:00', '11:00']));
    render(<TimeRuler hours={busy} initialStartTime="10:00" severalCourts={false} onSelectTime={onSelectTime} />);

    expect(screen.getByRole('button', { name: '10:00, Ocupado' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByText('Ocupado', { selector: 'p' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^Ver canchas/ })).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Ir al horario libre más cercano' }));

    expect(screen.getByRole('button', { name: '09:00' })).toHaveAttribute('aria-pressed', 'true');
    expect(onSelectTime).not.toHaveBeenCalled();
  });

  it('opens on the free hour nearest the middle when the middle is taken', () => {
    const busy = buildRulerHours(day(['08:00', '09:00', '10:00', '11:00', '12:00'], ['10:00']));
    render(<TimeRuler hours={busy} initialStartTime={null} severalCourts={false} onSelectTime={vi.fn()} />);

    expect(screen.getByRole('button', { name: '11:00' })).toHaveAttribute('aria-pressed', 'true');
  });
});
