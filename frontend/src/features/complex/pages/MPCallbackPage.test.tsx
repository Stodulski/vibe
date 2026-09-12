import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

vi.mock('@/shared/hooks/usePageTitle', () => ({ usePageTitle: vi.fn() }));
vi.mock('@/shared/stores', () => ({
  useStore: () => ({ setSelectedComplexId: vi.fn() }),
}));
vi.mock('@/shared/lib/ky', () => ({
  default: { post: vi.fn().mockReturnValue({ json: vi.fn() }) },
  withSignal: (signal?: AbortSignal) => (signal ? { signal } : {}),
}));
vi.mock('@/shared/components/layout/AppHeader', () => ({
  AppHeader: () => <header data-testid="app-header">Header</header>,
}));
vi.mock('@/shared/components/common/LoadingSpinner', () => ({
  LoadingSpinner: () => <div role="status">Loading</div>,
}));

function createWrapper(initialEntries: string[] = ['/settings/mp/callback']) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={initialEntries}>{children}</MemoryRouter>
    </QueryClientProvider>
  );
}

describe('MPCallbackPage', () => {
  it('shows error state when no code param is present', async () => {
    const Page = (await import('./MPCallbackPage')).default;
    render(<Page />, { wrapper: createWrapper() });
    // Without code/state params, component immediately goes to error state
    expect(screen.getByText(/error al conectar/i)).toBeInTheDocument();
  });

  it('renders AppHeader', async () => {
    const Page = (await import('./MPCallbackPage')).default;
    render(<Page />, { wrapper: createWrapper() });
    expect(screen.getByTestId('app-header')).toBeInTheDocument();
  });
});
