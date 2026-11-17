import { describe, it, expect, vi } from 'vitest';
import { renderWithProviders as render, screen, userEvent, within } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatEventTimeRange } from '@/shared/lib/eventDates';
import { formatPrice } from '@/shared/lib/utils';
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

  it('shows a cancelled event in the cancelled red, beside its title', () => {
    render(<EventCard event={{ ...EVENT, status: 'cancelled' }} onSelect={vi.fn()} />);
    expect(screen.getByText(t.cancelledBadge)).toHaveClass('text-error-text');
  });

  it('labels the row as a cancha abierta of its sport, so it reads apart from a tournament in the one list', () => {
    render(<EventCard event={EVENT} onSelect={vi.fn()} />);
    expect(screen.getByText(t.openPlayTitle('pádel'))).toBeInTheDocument();
  });

  it('titles the row with who can play, and says it is open to everyone without a modality', () => {
    render(<EventCard event={{ ...EVENT, modality: null, level_min: null, level_max: null }} onSelect={vi.fn()} />);
    expect(screen.getByText(t.openToEveryone)).toBeInTheDocument();
  });
});

describe('EventCard: the date block, the meta line and the press', () => {
  it('puts the date block before the kicker and the title in the DOM order', () => {
    render(<EventCard event={EVENT} onSelect={vi.fn()} />);

    const row = screen.getByRole('button');
    const time = row.querySelector('time');
    if (!time) throw new Error('expected a date block');
    const kicker = screen.getByText(t.openPlayTitle('pádel'));
    expect(time.compareDocumentPosition(kicker) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(time).toHaveAttribute('datetime', '2099-01-02');
  });

  it('writes the day of the date block as the number and the short month', () => {
    render(<EventCard event={EVENT} onSelect={vi.fn()} />);

    const time = screen.getByRole('button').querySelector('time');
    if (!time) throw new Error('expected a date block');
    expect(within(time).getByText('2')).toBeInTheDocument();
    expect(within(time).getByText('ene')).toBeInTheDocument();
  });

  it('shows the meta tokens: the hours, the places taken, the price and the court names', () => {
    render(<EventCard event={EVENT} onSelect={vi.fn()} />);

    const row = screen.getByRole('button');
    expect(within(row).getByText(formatEventTimeRange('19:00', '21:00'))).toBeInTheDocument();
    expect(within(row).getByText(t.spotsTaken(3, 8))).toBeInTheDocument();
    expect(within(row).getByText(formatPrice(EVENT.price))).toBeInTheDocument();
    expect(within(row).getByText(/Cancha 1/)).toBeInTheDocument();
  });

  it('counts the courts instead of naming them when there are more than two', () => {
    const courts = [
      { id: 'ct1', name: 'Pádel 1' },
      { id: 'ct2', name: 'Pádel 2' },
      { id: 'ct3', name: 'Pádel 3' },
    ];
    render(<EventCard event={{ ...EVENT, courts }} onSelect={vi.fn()} />);

    const row = screen.getByRole('button');
    expect(within(row).getByText(t.courtsCount(3))).toBeInTheDocument();
    expect(within(row).queryByText(/Pádel 1/)).not.toBeInTheDocument();
  });

  it('opens the detail of its event when the row is pressed', async () => {
    const onSelect = vi.fn();
    render(<EventCard event={EVENT} onSelect={onSelect} />);
    const user = userEvent.setup();

    await user.click(screen.getByRole('button'));

    expect(onSelect).toHaveBeenCalledWith('ev1');
  });
});
