import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import type { ComponentType } from 'react';

vi.mock('@/features/auth/api/auth.api', () => ({
  authApi: {
    resetPassword: vi.fn().mockResolvedValue({}),
    logout: vi.fn().mockResolvedValue({}),
  },
}));

vi.mock('@/shared/stores', () => ({
  useStore: () => ({ logout: vi.fn() }),
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

vi.mock('@/features/auth/schemas/auth.schema', () => ({
  resetPasswordSchema: { parse: vi.fn() },
}));

vi.mock('@hookform/resolvers/zod', () => ({
  zodResolver: () => vi.fn(),
}));

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}));

function renderPage(Page: ComponentType, initialEntries?: string[]) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter {...(initialEntries !== undefined ? { initialEntries } : {})}>
        <Page />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('ResetPasswordPage', () => {
  it('renders error state when no token is present', async () => {
    const ResetPasswordPage = (await import('./ResetPasswordPage')).default;
    renderPage(ResetPasswordPage);
    expect(screen.getByText('Algo salió mal')).toBeInTheDocument();
    expect(screen.getByText(/enlace es inválido/)).toBeInTheDocument();
  });

  it('renders form when token is present', async () => {
    const ResetPasswordPage = (await import('./ResetPasswordPage')).default;
    renderPage(ResetPasswordPage, ['/reset-password?token=valid-token']);
    expect(screen.getAllByText('Nueva contraseña').length).toBeGreaterThanOrEqual(1);
  });

  it('renders link to forgot password on error state', async () => {
    const ResetPasswordPage = (await import('./ResetPasswordPage')).default;
    renderPage(ResetPasswordPage);
    expect(screen.getByText('Enviar enlace')).toBeInTheDocument();
  });

  it('renders go to login link on error state', async () => {
    const ResetPasswordPage = (await import('./ResetPasswordPage')).default;
    renderPage(ResetPasswordPage);
    expect(screen.getByText('Ir a iniciar sesión')).toBeInTheDocument();
  });
});
