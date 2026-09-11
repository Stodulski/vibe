import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { LoginForm } from './LoginForm';
import { renderWithProviders } from '@/test/test-utils';

const mockMutate = vi.fn();

vi.mock('../hooks/useLogin', () => ({
  useLogin: () => ({
    mutate: mockMutate,
    isPending: false,
    isError: false,
    error: null,
  }),
}));

vi.mock('@/shared/lib/env', () => ({ env: { VITE_TURNSTILE_SITE_KEY: 'test-site-key' } }));

interface CapturedTurnstileProps {
  onSuccess?: (token: string) => void;
}

let turnstileProps: CapturedTurnstileProps | null = null;

vi.mock('@marsidev/react-turnstile', () => ({
  Turnstile: vi.fn((props: CapturedTurnstileProps) => {
    turnstileProps = props;
    return <div data-testid="turnstile-widget" />;
  }),
}));

describe('LoginForm — Turnstile enabled', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    turnstileProps = null;
  });

  it('disables the submit button until the challenge is solved', async () => {
    const user = userEvent.setup();
    renderWithProviders(<LoginForm />);

    await user.type(screen.getByLabelText(/email/i), 'test@example.com');
    await user.type(screen.getByLabelText(/contrase.a/i, { selector: 'input' }), 'password123');

    expect(screen.getByRole('button', { name: /iniciar sesi.n/i })).toBeDisabled();
    expect(mockMutate).not.toHaveBeenCalled();
  });

  it('enables the submit button and includes the token once the challenge is solved', async () => {
    const user = userEvent.setup();
    renderWithProviders(<LoginForm />);

    await user.type(screen.getByLabelText(/email/i), 'test@example.com');
    await user.type(screen.getByLabelText(/contrase.a/i, { selector: 'input' }), 'password123');

    turnstileProps?.onSuccess?.('solved-token');

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /iniciar sesi.n/i })).not.toBeDisabled();
    });

    await user.click(screen.getByRole('button', { name: /iniciar sesi.n/i }));

    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalledWith({
        email: 'test@example.com',
        password: 'password123',
        turnstile_token: 'solved-token',
      });
    });
  });
});
