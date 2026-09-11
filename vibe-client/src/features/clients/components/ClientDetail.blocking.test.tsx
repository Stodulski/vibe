import { describe, it, expect, vi, beforeEach } from 'vitest';
import userEvent from '@testing-library/user-event';
import { renderWithProviders, screen } from '@/test/test-utils';
import { ClientDetail } from './ClientDetail';
import { mockClient, defaultProps } from './ClientDetail.test';

/**
 * Block, unblock and WhatsApp are no longer buttons on the sheet itself —
 * they live behind the header's actions menu, so a test has to open it the
 * way a person would before the items exist in the document at all.
 */
async function openActionsMenu() {
  await userEvent.click(screen.getByRole('button', { name: /más acciones/i }));
}

vi.mock('../hooks/useClient', () => ({
  useClient: () => ({
    data: null,
    isLoading: false,
  }),
}));

vi.mock('../hooks/useUpdateClient', () => ({
  useUpdateClient: () => ({
    mutate: vi.fn(),
    isPending: false,
  }),
}));

describe('ClientDetail blocking and notes', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('offers blocking in the actions menu for a client who is not blocked', async () => {
    renderWithProviders(<ClientDetail {...defaultProps} />);
    await openActionsMenu();
    expect(screen.getByRole('menuitem', { name: /^bloquear/i })).toBeInTheDocument();
  });

  it('offers unblocking in the actions menu for a blocked client', async () => {
    const blockedClient = { ...mockClient, is_blocked: true };
    renderWithProviders(<ClientDetail {...defaultProps} client={blockedClient} />);
    await openActionsMenu();
    expect(screen.getByRole('menuitem', { name: /desbloquear/i })).toBeInTheDocument();
  });

  it('renders blocked badge for blocked client', () => {
    const blockedClient = { ...mockClient, is_blocked: true };
    renderWithProviders(<ClientDetail {...defaultProps} client={blockedClient} />);
    expect(screen.getByText('Bloqueado')).toBeInTheDocument();
  });

  it('returns null when client is null', () => {
    const { container } = renderWithProviders(<ClientDetail {...defaultProps} client={null} />);
    expect(container.querySelector('[role="dialog"]')).not.toBeInTheDocument();
  });

  it('renders notes textarea with existing notes', () => {
    renderWithProviders(<ClientDetail {...defaultProps} />);
    expect(screen.getByDisplayValue('Cliente frecuente')).toBeInTheDocument();
  });

  it('offers whatsapp in the actions menu', async () => {
    renderWithProviders(<ClientDetail {...defaultProps} />);
    await openActionsMenu();
    expect(screen.getByRole('menuitem', { name: /whatsapp/i })).toBeInTheDocument();
  });
});
