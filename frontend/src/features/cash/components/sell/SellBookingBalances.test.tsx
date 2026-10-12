import { describe, it, expect, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { makeBalanceDue } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import { SellBookingBalances } from './SellBookingBalances';

const t = ES_AR;

const juan = makeBalanceDue({
  booking_id: 'b1',
  court_name: 'Cancha 2',
  start_time: '19:00',
  end_time: '20:30',
  client_name: 'Juan Pérez',
  remaining: 350000,
});
const ana = makeBalanceDue({
  booking_id: 'b2',
  court_name: 'Cancha 1',
  start_time: '21:00',
  end_time: '22:00',
  client_name: 'Ana Gómez',
  remaining: 120000,
});

function renderSection(overrides: Partial<Parameters<typeof SellBookingBalances>[0]> = {}) {
  const props = {
    bookings: [juan, ana],
    inCartIds: new Set<string>(),
    isLoading: false,
    isError: false,
    onAdd: vi.fn(),
    onRetry: vi.fn(),
    ...overrides,
  };
  render(<SellBookingBalances {...props} />);
  return props;
}

describe('SellBookingBalances', () => {
  it('shows each booking with its court, time range, client and what it still owes', () => {
    renderSection();

    expect(screen.getByRole('heading', { name: t.cash.bookingsTitle })).toBeInTheDocument();
    const tile = screen.getByRole('button', { name: /Juan Pérez/ });
    expect(within(tile).getByText('Cancha 2')).toBeInTheDocument();
    expect(within(tile).getByText('19:00 a 20:30')).toBeInTheDocument();
    expect(within(tile).getByText(`${t.cash.bookingRemaining} $3.500`)).toBeInTheDocument();
  });

  it('shows the empty state when no booking owes money today', () => {
    renderSection({ bookings: [] });

    expect(screen.getByText('No hay turnos con saldo pendiente hoy.')).toBeInTheDocument();
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('adds the booking when its tile is tapped', async () => {
    const user = userEvent.setup();
    const { onAdd } = renderSection();

    await user.click(screen.getByRole('button', { name: /Ana Gómez/ }));

    expect(onAdd).toHaveBeenCalledWith(ana);
  });

  it('disables a booking already in the cart and says so, so it cannot be added twice', () => {
    renderSection({ inCartIds: new Set(['b1']) });

    const tile = screen.getByRole('button', { name: /Juan Pérez/ });
    expect(tile).toBeDisabled();
    expect(within(tile).getByText(t.cash.bookingInCart)).toBeInTheDocument();
  });

  it('shows the load error with a retry while there is nothing to show', async () => {
    const user = userEvent.setup();
    const { onRetry } = renderSection({ bookings: [], isError: true });

    expect(screen.getByText(t.cash.bookingsLoadError)).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: t.layout.retry }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });
});

describe('SellBookingBalances — the row below lg', () => {
  it('lists the bookings as one named list, so a screen reader reads how many there are', () => {
    renderSection();

    const list = screen.getByRole('list', { name: t.cash.bookingsTitle });
    expect(within(list).getAllByRole('listitem')).toHaveLength(2);
    expect(screen.getByRole('region', { name: t.cash.bookingsTitle })).toBeInTheDocument();
  });

  it('keeps the same list while loading, so the row does not swap elements when the bookings arrive', () => {
    renderSection({ bookings: [], isLoading: true });

    const list = screen.getByRole('list', { name: t.cash.bookingsTitle });
    expect(list).toHaveAttribute('aria-busy', 'true');
  });

  it('reaches each booking in order with the keyboard', async () => {
    const user = userEvent.setup();
    renderSection();

    await user.tab();
    expect(screen.getByRole('button', { name: /Cancha 2/ })).toHaveFocus();
    await user.tab();
    expect(screen.getByRole('button', { name: /Cancha 1/ })).toHaveFocus();
  });

  it('brings a booking that takes keyboard focus fully into the row, even when only its edge is visible', async () => {
    const user = userEvent.setup();
    const scrollIntoView = vi.fn();
    // jsdom has no layout and no scrollIntoView; the test only asks that it is called.
    Element.prototype.scrollIntoView = scrollIntoView;
    try {
      renderSection();

      await user.tab();

      expect(scrollIntoView).toHaveBeenCalledWith({ block: 'nearest', inline: 'nearest' });
      expect(screen.getByRole('button', { name: /Cancha 2/ })).toHaveFocus();
    } finally {
      Reflect.deleteProperty(Element.prototype, 'scrollIntoView');
    }
  });
});
