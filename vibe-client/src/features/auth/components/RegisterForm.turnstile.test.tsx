import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { RegisterForm } from './RegisterForm';
import { renderWithProviders } from '@/test/test-utils';

const mockMutate = vi.fn();

vi.mock('../hooks/useRegister', () => ({
  useRegister: () => ({
    mutate: mockMutate,
    isPending: false,
    isError: false,
    error: null,
  }),
}));

vi.mock('../api/leads.api', () => ({
  captureAbandonedRegistrationLead: vi.fn(),
  captureAbandonedRegistrationLeadBeacon: vi.fn(),
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

/** Fills steps 1 and 2 and lands on step 3 (password + Turnstile + submit). */
async function reachStep3(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText(/email/i), 'juan@test.com');
  await user.click(screen.getByRole('button', { name: /siguiente/i }));
  await waitFor(() => {
    expect(screen.getByLabelText(/^nombre$/i)).toBeInTheDocument();
  });

  await user.type(screen.getByLabelText(/^nombre$/i), 'Juan');
  await user.type(screen.getByLabelText(/apellido/i), 'Perez');
  await user.type(screen.getByLabelText(/tel.fono/i), '1123456789');
  await user.click(screen.getByRole('button', { name: /siguiente/i }));
  await waitFor(() => {
    expect(screen.getByLabelText(/^contrase.a$/i)).toBeInTheDocument();
  });

  await user.type(screen.getByLabelText(/^contrase.a$/i), 'password123');
  await user.type(screen.getByLabelText(/confirmar contrase.a/i), 'password123');
}

describe('RegisterForm — Turnstile enabled (step 3)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    turnstileProps = null;
  });

  it('disables the create-account button until the challenge is solved', async () => {
    const user = userEvent.setup();
    renderWithProviders(<RegisterForm />);
    await reachStep3(user);

    expect(screen.getByRole('button', { name: /crear cuenta/i })).toBeDisabled();
    expect(mockMutate).not.toHaveBeenCalled();
  });

  it('enables the button and includes the token in the submitted payload once solved', async () => {
    const user = userEvent.setup();
    renderWithProviders(<RegisterForm />);
    await reachStep3(user);

    turnstileProps?.onSuccess?.('solved-token');

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /crear cuenta/i })).not.toBeDisabled();
    });

    await user.click(screen.getByRole('button', { name: /crear cuenta/i }));

    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalledWith({
        first_name: 'Juan',
        last_name: 'Perez',
        email: 'juan@test.com',
        phone: '+541123456789',
        password: 'password123',
        turnstile_token: 'solved-token',
      });
    });
  });
});
