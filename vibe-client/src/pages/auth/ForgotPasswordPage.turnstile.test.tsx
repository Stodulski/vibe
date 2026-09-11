import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { authApi } from '@/features/auth';

vi.mock('@/features/auth/api/auth.api', () => ({
  authApi: { forgotPassword: vi.fn().mockResolvedValue({}) },
}));

vi.mock('@/shared/components/layout/AppHeader', () => ({
  AppHeader: ({ className }: { className?: string }) => <header data-testid="app-header" className={className} />,
}));

vi.mock('@/shared/hooks/usePageTitle', () => ({ usePageTitle: vi.fn() }));

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

async function renderPage() {
  const ForgotPasswordPage = (await import('./ForgotPasswordPage')).default;
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <ForgotPasswordPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('ForgotPasswordPage — Turnstile enabled', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    turnstileProps = null;
  });

  it('disables the send button until the challenge is solved', async () => {
    const user = userEvent.setup();
    await renderPage();

    await user.type(screen.getByLabelText('Email'), 'alguien@ejemplo.com');

    expect(screen.getByRole('button', { name: /enviar enlace/i })).toBeDisabled();
    expect(authApi.forgotPassword).not.toHaveBeenCalled();
  });

  it('enables the button and sends the token once solved', async () => {
    const user = userEvent.setup();
    await renderPage();

    await user.type(screen.getByLabelText('Email'), 'alguien@ejemplo.com');
    turnstileProps?.onSuccess?.('solved-token');

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /enviar enlace/i })).not.toBeDisabled();
    });
    await user.click(screen.getByRole('button', { name: /enviar enlace/i }));

    await waitFor(() => {
      expect(authApi.forgotPassword).toHaveBeenCalledWith('alguien@ejemplo.com', 'solved-token');
    });
  });
});
