import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen, userEvent } from '@/test/test-utils';
import { SecurityForm } from './SecurityForm';

vi.mock('@/shared/stores', () => ({
  useStore: Object.assign(() => ({}), {
    getState: () => ({
      logout: vi.fn(),
    }),
  }),
}));

vi.mock('../api/auth.api', () => ({
  authApi: {
    updateMe: vi.fn(),
  },
}));

vi.mock('./DeleteAccountSection', () => ({
  DeleteAccountSection: () => <div data-testid="delete-account-section" />,
}));

describe('SecurityForm', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows validation error when submitting empty form', async () => {
    const user = userEvent.setup();
    renderWithProviders(<SecurityForm />);

    await user.click(screen.getByRole('button', { name: /cambiar contrase.a/i }));

    expect(await screen.findAllByText(/campo requerido/i)).toHaveLength(2);
  });

  it('shows min length error for short new password', async () => {
    const user = userEvent.setup();
    renderWithProviders(<SecurityForm />);

    await user.type(screen.getByLabelText(/contrase.a actual/i), 'oldpass123');
    await user.type(screen.getByLabelText(/^nueva contrase.a$/i), 'short');
    await user.type(screen.getByLabelText(/confirmar nueva contrase.a/i), 'short');
    await user.click(screen.getByRole('button', { name: /cambiar contrase.a/i }));

    expect(await screen.findByText(/m.nimo 8 caracteres/i)).toBeInTheDocument();
  });

  it('renders the delete account section', () => {
    renderWithProviders(<SecurityForm />);
    expect(screen.getByTestId('delete-account-section')).toBeInTheDocument();
  });
});
