import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen, userEvent } from '@/test/test-utils';
import { DeleteAccountSection } from './DeleteAccountSection';

vi.mock('@/shared/stores', () => ({
  useStore: Object.assign(() => ({}), {
    getState: () => ({
      logout: vi.fn(),
    }),
  }),
}));

vi.mock('../api/auth.api', () => ({
  authApi: {
    deleteAccount: vi.fn(),
  },
}));

/**
 * Deleting the account is behind the overflow menu beside "Cambiar
 * contraseña" now, not a red panel of its own at the bottom of the screen.
 *
 * These used to assert a title, a description and a visible button — the
 * furniture of that panel. What matters is unchanged and is what is checked
 * here: the action is reachable, it is not shouting before anyone asked for
 * it, and it cannot fire without a confirmation.
 */
describe('DeleteAccountSection', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const openMenu = async () => {
    const user = userEvent.setup();
    renderWithProviders(<DeleteAccountSection />);
    await user.click(screen.getByRole('button', { name: /acciones/i }));
    return user;
  };

  it('keeps the action out of sight until the menu is opened', () => {
    renderWithProviders(<DeleteAccountSection />);
    // Friction before a destructive step is LESS prominence, not more: one
    // interaction further away, and no red until the confirmation asks.
    expect(screen.queryByText(/eliminar cuenta/i)).not.toBeInTheDocument();
  });

  it('offers the action inside the menu', async () => {
    await openMenu();
    expect(await screen.findByRole('menuitem', { name: /eliminar cuenta/i })).toBeInTheDocument();
  });

  it('confirms before deleting anything', async () => {
    const user = await openMenu();

    await user.click(await screen.findByRole('menuitem', { name: /eliminar cuenta/i }));

    const dialog = await screen.findByRole('alertdialog');
    expect(dialog).toBeInTheDocument();
    expect(await screen.findByRole('button', { name: /cancelar/i })).toBeInTheDocument();
  });
});
