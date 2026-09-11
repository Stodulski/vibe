import { describe, it, expect } from 'vitest';
import { renderWithProviders, screen, userEvent, waitFor } from '@/test/test-utils';
import { CommandPalette } from './CommandPalette';

describe('CommandPalette', () => {
  it('does not render dialog by default', () => {
    renderWithProviders(<CommandPalette />);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('opens dialog on Ctrl+K', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CommandPalette />);

    await user.keyboard('{Control>}k{/Control}');

    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });
  });

  it('renders navigation items when open', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CommandPalette />);

    await user.keyboard('{Control>}k{/Control}');

    await waitFor(() => {
      expect(screen.getByText(/dashboard|panel/i)).toBeInTheDocument();
      expect(screen.getByText(/reservas/i)).toBeInTheDocument();
      expect(screen.getByText(/canchas/i)).toBeInTheDocument();
      expect(screen.getByText(/clientes/i)).toBeInTheDocument();
    });
  });

  it('renders search input when open', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CommandPalette />);

    await user.keyboard('{Control>}k{/Control}');

    await waitFor(() => {
      expect(screen.getByRole('combobox')).toBeInTheDocument();
    });
  });

  it('opens via custom event', async () => {
    renderWithProviders(<CommandPalette />);

    window.dispatchEvent(new Event('open-command-palette'));

    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });
  });
});
