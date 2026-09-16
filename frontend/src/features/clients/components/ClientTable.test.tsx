import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ClientTable } from './ClientTable';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Client } from '@/shared/types/api.types';

function makeClient(overrides: Partial<Client> = {}): Client {
  return {
    id: 'cl1',
    complex_id: 'c1',
    first_name: 'Juan',
    last_name: 'Perez',
    phone: '+541155550000',
    email: null,
    is_blocked: false,
    total_bookings: 10,
    no_shows: 2,
    created_at: '2026-03-14T10:00:00Z',
    updated_at: '2026-03-14T10:00:00Z',
    ...overrides,
  };
}

function renderTable(clients: Client[], overrides: Partial<Parameters<typeof ClientTable>[0]> = {}) {
  const props = { clients, onSelectClient: vi.fn(), onBlockClient: vi.fn(), ...overrides };
  render(<ClientTable {...props} />);
  return props;
}

describe('ClientTable', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders as a table with one row per client', () => {
    renderTable([makeClient(), makeClient({ id: 'cl2', first_name: 'Ana' })]);
    expect(screen.getByRole('table')).toBeInTheDocument();
    expect(screen.getByText('Juan Perez')).toBeInTheDocument();
    expect(screen.getByText('Ana Perez')).toBeInTheDocument();
  });

  it('states bookings, no-shows and attendance for a client', () => {
    renderTable([makeClient({ total_bookings: 10, no_shows: 2 })]);
    expect(screen.getByText('10')).toBeInTheDocument();
    expect(screen.getByText('2')).toBeInTheDocument();
    expect(screen.getByText('80%')).toBeInTheDocument();
  });

  it('shows an em dash for a client with no email', () => {
    renderTable([makeClient({ email: null })]);
    expect(screen.getByText('—')).toBeInTheDocument();
  });

  it('shows the email when the client has one', () => {
    renderTable([makeClient({ email: 'juan@test.com' })]);
    expect(screen.getByText('juan@test.com')).toBeInTheDocument();
  });

  it('shows the month and year the client joined', () => {
    renderTable([makeClient({ created_at: '2026-03-14T10:00:00Z' })]);
    expect(screen.getByText('mar 2026')).toBeInTheDocument();
  });

  // The owner explicitly asked for no blocked indicator in the table, unlike
  // the card, which still shows a `Ban` icon.
  it('renders no blocked badge for a blocked client', () => {
    renderTable([makeClient({ is_blocked: true })]);
    expect(screen.queryByLabelText(ES_AR.clients.blocked)).not.toBeInTheDocument();
  });

  it('opens the detail when the client name is activated', async () => {
    const user = userEvent.setup();
    const props = renderTable([makeClient()]);

    await user.click(screen.getByRole('button', { name: 'Juan Perez' }));

    expect(props.onSelectClient).toHaveBeenCalledWith(expect.objectContaining({ id: 'cl1' }));
  });

  it('the ⋯ menu offers WhatsApp, call and block without opening the detail', async () => {
    const user = userEvent.setup();
    const props = renderTable([makeClient()]);

    await user.click(screen.getByRole('button', { name: ES_AR.common.rowActionsLabel }));
    expect(await screen.findByText(ES_AR.clients.whatsapp)).toBeInTheDocument();
    expect(screen.getByText(ES_AR.clients.callClient)).toBeInTheDocument();

    await user.click(screen.getByText(ES_AR.clients.block));

    expect(props.onBlockClient).toHaveBeenCalledWith(expect.objectContaining({ id: 'cl1' }));
    expect(props.onSelectClient).not.toHaveBeenCalled();
  });

  it('offers unblocking for a client who is already blocked', async () => {
    const user = userEvent.setup();
    renderTable([makeClient({ is_blocked: true })]);

    await user.click(screen.getByRole('button', { name: ES_AR.common.rowActionsLabel }));

    expect(await screen.findByText(ES_AR.clients.unblock)).toBeInTheDocument();
  });
});
