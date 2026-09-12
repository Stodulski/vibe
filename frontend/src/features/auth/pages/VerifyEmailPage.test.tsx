import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';

const mockVerifyEmail = vi.fn<(token: string) => Promise<{ message: string }>>();
const mockLogout = vi.fn();

vi.mock('@/features/auth/api/auth.api', () => ({
  authApi: {
    verifyEmail: (token: string) => mockVerifyEmail(token),
    logout: vi.fn().mockResolvedValue({}),
  },
}));

vi.mock('@/shared/stores', () => ({
  useStore: () => ({ logout: mockLogout }),
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

function createTestQueryClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

async function renderPage(entries: string[], client: QueryClient = createTestQueryClient()) {
  const VerifyEmailPage = (await import('./VerifyEmailPage')).default;
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={entries}>
        <VerifyEmailPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('VerifyEmailPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows loading state initially with a token', async () => {
    mockVerifyEmail.mockReturnValue(
      new Promise(() => {
        /* never resolves */
      }),
    );
    await renderPage(['/verify-email?token=test-token']);
    expect(screen.getByText('Verificando...')).toBeInTheDocument();
  });

  it('shows error state when no token', async () => {
    await renderPage(['/verify-email']);
    expect(screen.getByText('Algo salió mal')).toBeInTheDocument();
  });

  it('shows success state after verification', async () => {
    mockVerifyEmail.mockResolvedValue({ message: 'ok' });
    await renderPage(['/verify-email?token=valid-token']);
    await waitFor(() => {
      expect(screen.getByText('Email verificado correctamente')).toBeInTheDocument();
    });
  });

  it('shows error state when verification fails', async () => {
    mockVerifyEmail.mockRejectedValue(new Error('Invalid token'));
    await renderPage(['/verify-email?token=bad-token']);
    await waitFor(() => {
      expect(screen.getByText('Algo salió mal')).toBeInTheDocument();
    });
  });

  // The old implementation fired the (single-use) verification POST from a
  // bare `useEffect` with no cache — every mount re-sent it unconditionally,
  // including a remount that happens for reasons unrelated to a fresh visit
  // (e.g. React StrictMode's mount→unmount→mount in development, or a parent
  // re-render). A second POST with an already-consumed token can come back
  // "already used" and flip a real success into an error. Keying the query
  // on the token with `staleTime: Infinity` makes a remount reuse the cached
  // result instead of consuming the token again.
  it('does not re-verify an already-consumed token when the hook remounts', async () => {
    mockVerifyEmail.mockResolvedValue({ message: 'ok' });
    const client = createTestQueryClient();

    const { unmount } = await renderPage(['/verify-email?token=once-only-token'], client);
    await waitFor(() => {
      expect(screen.getByText('Email verificado correctamente')).toBeInTheDocument();
    });
    unmount();

    await renderPage(['/verify-email?token=once-only-token'], client);
    expect(screen.getByText('Email verificado correctamente')).toBeInTheDocument();
    expect(mockVerifyEmail).toHaveBeenCalledTimes(1);
  });
});
