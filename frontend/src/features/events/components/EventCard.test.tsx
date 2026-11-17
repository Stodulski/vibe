import { describe, it, expect, vi } from 'vitest';
import { renderWithProviders as render, screen } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { EventCard } from './EventCard';
import type { OpenPlayEvent } from '@/shared/types/api.types';

const t = ES_AR.events;

const EVENT: OpenPlayEvent = {
  id: 'ev1',
  complex_id: 'c1',
  name: 'Americano de viernes',
  sport: 'padel',
  date: '2099-01-02',
  start_time: '19:00',
  end_time: '21:00',
  duration_minutes: 120,
  capacity: 8,
  price: 500000,
  status: 'active',
  courts: [{ id: 'ct1', name: 'Cancha 1' }],
  confirmed_count: 3,
  held_count: 0,
  spots_left: 5,
  created_at: '2026-11-14T12:00:00Z',
};

describe('EventCard', () => {
  it('lines the state word up with the name by their baselines', () => {
    render(<EventCard event={{ ...EVENT, spots_left: 0 }} onSelect={vi.fn()} />);

    const row = screen.getByText(t.fullBadge).parentElement;
    expect(row).toHaveClass('items-baseline');
  });

  it('shows a full event in the green of the other done states', () => {
    render(<EventCard event={{ ...EVENT, spots_left: 0 }} onSelect={vi.fn()} />);
    expect(screen.getByText(t.fullBadge)).toHaveClass('text-success-text');
  });

  it('shows a cancelled event in the cancelled red', () => {
    render(<EventCard event={{ ...EVENT, status: 'cancelled' }} onSelect={vi.fn()} />);
    expect(screen.getByText(t.cancelledBadge)).toHaveClass('text-error-text');
  });

  it('labels the card as a cancha abierta of its sport, so it reads apart from a tournament in the one list', () => {
    render(<EventCard event={EVENT} onSelect={vi.fn()} />);
    expect(screen.getByText(t.openPlayTitle('pádel'))).toBeInTheDocument();
  });

  it('titles the card with who can play, and says it is open to everyone without a modality', () => {
    render(<EventCard event={{ ...EVENT, modality: null, level_min: null, level_max: null }} onSelect={vi.fn()} />);
    expect(screen.getByText(t.openToEveryone)).toBeInTheDocument();
  });

  it('capitalises the weekday on a block of its own, not on an inline span', () => {
    render(<EventCard event={EVENT} onSelect={vi.fn()} />);
    expect(screen.getByText('viernes 2 de enero')).toHaveClass('inline-block', 'first-letter:uppercase');
  });
});
