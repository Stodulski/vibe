import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { toast } from 'sonner';
import { authApi } from '@/features/auth';
import { ES_AR } from '@/shared/i18n/es_AR';
import { makeConsumedHttpError } from '@/test/factories';

vi.mock('@/features/auth/api/auth.api', () => ({
  authApi: { forgotPassword: vi.fn().mockResolvedValue({}) },
}));

vi.mock('@/shared/components/layout/AppHeader', () => ({
  AppHeader: ({ className }: { className?: string }) => (
    <header data-testid="app-header" className={className}>
      Header
    </header>
  ),
}));

vi.mock('@/shared/hooks/usePageTitle', () => ({
  usePageTitle: vi.fn(),
}));

vi.mock('@/shared/components/ui/button', async () => {
  const { passthrough } = await import('@/test/ui-mocks');
  return { Button: passthrough('button') };
});

vi.mock('@/shared/components/ui/input', async () => {
  const { passthrough } = await import('@/test/ui-mocks');
  return { Input: passthrough('input') };
});

vi.mock('@/shared/components/ui/label', async () => {
  const { passthrough } = await import('@/test/ui-mocks');
  return { Label: passthrough('label') };
});

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}));

describe('ForgotPasswordPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  async function submitEmail(email = 'alguien@ejemplo.com') {
    const user = userEvent.setup();
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
    await user.type(screen.getByLabelText('Email'), email);
    await user.click(screen.getByRole('button', { name: ES_AR.auth.forgotPasswordSend }));
  }

  it('sends the reset link for the address that was typed', async () => {
    await submitEmail('alguien@ejemplo.com');

    await waitFor(() => {
      expect(authApi.forgotPassword).toHaveBeenCalledWith('alguien@ejemplo.com');
    });
  });

  it('confirms it was sent when the request succeeds', async () => {
    await submitEmail();

    expect(await screen.findByText(ES_AR.auth.forgotPasswordSuccess)).toBeInTheDocument();
  });

  // The one that matters. The page answers "we sent it" even when the server
  // says the address does not exist, because any other answer tells a stranger
  // which emails are registered here. A refactor that surfaced the error would
  // leak that, and every assertion about the page's copy would stay green —
  // which is exactly how this file used to be written.
  it('still confirms it was sent when the request fails, so no address is confirmed to exist', async () => {
    vi.mocked(authApi.forgotPassword).mockRejectedValueOnce(await makeConsumedHttpError(404, {}));

    await submitEmail('no-existe@ejemplo.com');

    expect(await screen.findByText(ES_AR.auth.forgotPasswordSuccess)).toBeInTheDocument();
    expect(toast.error).not.toHaveBeenCalled();
  });

  // Rate limiting is the one failure worth telling the truth about: it says
  // nothing about whether the address exists, and silence would leave someone
  // pressing a button that is doing nothing.
  it('reports rate limiting instead, and does not claim the mail was sent', async () => {
    vi.mocked(authApi.forgotPassword).mockRejectedValueOnce(await makeConsumedHttpError(429, {}));

    await submitEmail();

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(ES_AR.auth.rateLimitError);
    });
    expect(screen.queryByText(ES_AR.auth.forgotPasswordSuccess)).not.toBeInTheDocument();
  });

  // The old catch block cast the rejection to `{ response?: { status?: number } }`
  // and read `.response?.status` straight off it — safe with optional
  // chaining only when the rejection itself is an object. A rejection that
  // isn't (e.g. `undefined`, which a dropped fetch can produce) made that
  // cast throw *inside* the catch handler, before `setSent(true)` ran: the
  // form went back to idle with no confirmation and no error, looking broken.
  it('still confirms it was sent when the rejection carries no response shape at all', async () => {
    vi.mocked(authApi.forgotPassword).mockRejectedValueOnce(undefined);

    await submitEmail();

    expect(await screen.findByText(ES_AR.auth.forgotPasswordSuccess)).toBeInTheDocument();
    expect(toast.error).not.toHaveBeenCalled();
  });
});
