import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ClientInfoRow } from './ClientInfoRow';
import { renderWithProviders } from '@/test/test-utils';
import { makeBooking } from '@/test/factories';
import type { Booking, Client } from '@/shared/types/api.types';

function createBooking(overrides?: Partial<Booking>): Booking {
  return makeBooking({
    id: 'b1',
    complex_id: 'c1',
    court_id: 'ct1',
    client_id: 'cl1',
    court_name: 'Cancha 1',
    client_name: 'Cliente Anonimo',
    client_phone: '1155550000',
    date: '2026-03-16',
    start_time: '10:00',
    duration_minutes: 90,
    price: 15000,
    deposit_amount: 0,
    status: 'confirmed',
    collection_status: 'fully_paid',
    refund_status: 'none',
    notes: '',
    reminder_sent_2h: false,
    created_at: '2026-03-14T10:00:00Z',
    updated_at: '2026-03-14T10:00:00Z',
    ...overrides,
  });
}

function createClient(overrides?: Partial<Client>): Client {
  return {
    id: 'cl1',
    complex_id: 'c1',
    first_name: 'Juan',
    last_name: 'Perez',
    phone: '1155550000',
    is_blocked: false,
    total_bookings: 3,
    no_shows: 0,
    created_at: '2026-03-14T10:00:00Z',
    updated_at: '2026-03-14T10:00:00Z',
    ...overrides,
  };
}

describe('ClientInfoRow', () => {
  it('renders a button with a chevron and fires onOpenClient when a client is present', async () => {
    const user = userEvent.setup();
    const client = createClient();
    const onOpenClient = vi.fn();
    renderWithProviders(<ClientInfoRow booking={createBooking()} client={client} onOpenClient={onOpenClient} />);

    const button = screen.getByRole('button', { name: /juan perez/i });
    expect(button).toBeInTheDocument();
    expect(button.querySelector('svg')).toBeInTheDocument();

    await user.click(button);
    expect(onOpenClient).toHaveBeenCalledWith(client);
  });

  it('renders plain text when there is no resolved client', () => {
    const onOpenClient = vi.fn();
    renderWithProviders(
      <ClientInfoRow
        booking={createBooking({ client_name: 'Cliente Anonimo' })}
        client={undefined}
        onOpenClient={onOpenClient}
      />,
    );

    expect(screen.queryByRole('button')).not.toBeInTheDocument();
    expect(screen.getByText('Cliente Anonimo')).toBeInTheDocument();
  });

  it('renders plain text when onOpenClient is not provided, even with a resolved client', () => {
    renderWithProviders(<ClientInfoRow booking={createBooking()} client={createClient()} />);

    expect(screen.queryByRole('button')).not.toBeInTheDocument();
    expect(screen.getByText('Juan Perez')).toBeInTheDocument();
  });
});
