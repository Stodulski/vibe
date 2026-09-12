import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ClientGrid } from './ClientGrid';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Client } from '@/shared/types/api.types';

function makeClient(overrides: Partial<Client> = {}): Client {
  return {
    id: 'cl1',
    complex_id: 'c1',
    first_name: 'Juan',
    last_name: 'Perez',
    phone: '+541155550000',
    is_blocked: false,
    total_bookings: 10,
    no_shows: 2,
    created_at: '2026-03-14T10:00:00Z',
    updated_at: '2026-03-14T10:00:00Z',
    ...overrides,
  };
}

function renderGrid(clients: Client[], overrides: Partial<Parameters<typeof ClientGrid>[0]> = {}) {
  const props = {
    clients,
    onSelectClient: vi.fn(),
    onBlockClient: vi.fn(),
    ...overrides,
  };
  render(<ClientGrid {...props} />);
  return props;
}

describe('the clients grid', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows each client once', () => {
    renderGrid([makeClient(), makeClient({ id: 'cl2', first_name: 'Ana' })]);

    expect(screen.getByText('Juan Perez')).toBeInTheDocument();
    expect(screen.getByText('Ana Perez')).toBeInTheDocument();
  });

  // The card's body is the way into the detail, and it is one target: pressing
  // anywhere that is not the menu opens the drawer.
  it('opens the detail when the card is pressed', async () => {
    const user = userEvent.setup();
    const props = renderGrid([makeClient()]);

    await user.click(screen.getByRole('button', { name: 'Juan Perez' }));

    expect(props.onSelectClient).toHaveBeenCalledWith(expect.objectContaining({ id: 'cl1' }));
  });

  // Eight of ten kept, which is 80% — the boundary between the good tone and
  // the warning one, so it also pins which side of the line 80 falls on.
  it('states how many bookings the client has and how many they turned up for', () => {
    renderGrid([makeClient({ total_bookings: 10, no_shows: 2 })]);

    expect(screen.getByText('10')).toBeInTheDocument();
    expect(screen.getByText('80%')).toBeInTheDocument();
  });

  // A client who has never booked has missed nothing. Scoring them 0% would
  // file someone brand new beside someone who no-showed five times.
  it('counts a client with no bookings as fully attended', () => {
    renderGrid([makeClient({ total_bookings: 0, no_shows: 0 })]);

    expect(screen.getByText('100%')).toBeInTheDocument();
  });

  it('blocks from the card menu without opening the detail', async () => {
    const user = userEvent.setup();
    const props = renderGrid([makeClient()]);

    await user.click(screen.getByRole('button', { name: ES_AR.common.rowActionsLabel }));
    await user.click(await screen.findByText(ES_AR.clients.block));

    expect(props.onBlockClient).toHaveBeenCalledWith(expect.objectContaining({ id: 'cl1' }));
    expect(props.onSelectClient).not.toHaveBeenCalled();
  });

  it('offers unblocking for a client who is blocked', async () => {
    const user = userEvent.setup();
    renderGrid([makeClient({ is_blocked: true })]);

    await user.click(screen.getByRole('button', { name: ES_AR.common.rowActionsLabel }));

    expect(await screen.findByText(ES_AR.clients.unblock)).toBeInTheDocument();
  });
});
