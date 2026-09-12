import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';

vi.mock('@/features/auth/api/auth.api', () => ({
  authApi: { resendVerification: vi.fn().mockResolvedValue({}) },
}));

vi.mock('@/shared/components/layout/AppHeader', () => ({
  AppHeader: ({ className }: { className?: string }) => (
    <header data-testid="app-header" className={className}>
      Header
    </header>
  ),
}));

vi.mock('@/shared/components/ui/button', async () => {
  const { passthrough } = await import('@/test/ui-mocks');
  return { Button: passthrough('button') };
});

vi.mock('@/shared/hooks/usePageTitle', () => ({
  usePageTitle: vi.fn(),
}));

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

async function renderPage(state: unknown) {
  const VerifyEmailSentPage = (await import('./VerifyEmailSentPage')).default;
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[{ pathname: '/verify-email-sent', state }]}>
        <VerifyEmailSentPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

// The three tests that asserted the static title, the description and the
// login link are gone. They had no branch behind them: the copy is not
// conditional, so they could only fail on a wording change — which is a
// translation decision, not a regression. What is left exercises the page's
// actual logic: the email arriving via location state, and the resend path.
describe('VerifyEmailSentPage', () => {
  it('shows email when passed via location state', async () => {
    await renderPage({ email: 'test@example.com' });
    expect(screen.getByText('test@example.com')).toBeInTheDocument();
  });

  it('shows resend button when email is provided', async () => {
    await renderPage({ email: 'test@example.com' });
    expect(screen.getByText('Reenviar email')).toBeInTheDocument();
  });

  // `location.state` isn't guaranteed to match the shape this page expects —
  // an `as` cast used to trust it blindly, so a non-string `email` (e.g. a
  // stray number) would still read as truthy and get sent on to the resend
  // request as-is. safeParse rejects it instead, falling back to "no email".
  it('does not show the email or the resend button when location.state.email is not a string', async () => {
    await renderPage({ email: 12345 });
    expect(screen.queryByText('12345')).not.toBeInTheDocument();
    expect(screen.queryByText('Reenviar email')).not.toBeInTheDocument();
  });

  it('does not show the email or the resend button without location.state', async () => {
    await renderPage(null);
    expect(screen.queryByText('Reenviar email')).not.toBeInTheDocument();
  });
});
