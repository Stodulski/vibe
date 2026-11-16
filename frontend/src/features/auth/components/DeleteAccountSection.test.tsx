import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen, userEvent } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
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
 * Deleting the account is the "Zona de peligro" at the end of the security
 * tab: a title, one line and a visible destructive outline button, with no
 * overflow menu. It still cannot fire without a confirmation.
 */
describe('DeleteAccountSection', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows the danger zone with a visible destructive button and no menu', () => {
    renderWithProviders(<DeleteAccountSection />);

    expect(screen.getByRole('heading', { name: ES_AR.common.dangerZone })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: ES_AR.profile.deleteAccount })).toHaveAttribute(
      'data-variant',
      'destructive-outline',
    );
    expect(screen.queryByRole('button', { name: /acciones/i })).not.toBeInTheDocument();
  });

  it('confirms before deleting anything', async () => {
    const user = userEvent.setup();
    renderWithProviders(<DeleteAccountSection />);

    await user.click(screen.getByRole('button', { name: ES_AR.profile.deleteAccount }));

    expect(await screen.findByRole('alertdialog')).toBeInTheDocument();
    expect(await screen.findByRole('button', { name: /cancelar/i })).toBeInTheDocument();
  });
});
