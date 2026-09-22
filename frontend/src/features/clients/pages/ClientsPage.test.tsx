import { describe, it, expect, vi } from 'vitest';
import type { ComponentType } from 'react';
import { screen } from '@testing-library/react';
import { renderWithProviders } from '@/test/test-utils';

vi.mock('@/shared/hooks/usePageTitle', () => ({ usePageTitle: vi.fn() }));
vi.mock('@/shared/tour/useTourReady', () => ({ useTourReady: vi.fn() }));
vi.mock('@/shared/components/common/PageHeader', () => ({
  PageHeader: ({ title }: { title: string }) => <h1>{title}</h1>,
}));
vi.mock('@/shared/components/common/EmptyState', () => ({
  EmptyState: ({ title }: { title: string }) => <div data-testid="empty">{title}</div>,
}));
vi.mock('@/shared/components/common/Skeletons', () => ({
  SkeletonTable: () => <div data-testid="skeleton">Loading...</div>,
}));
vi.mock('@/shared/components/ui/input', async () => {
  const { passthrough } = await import('@/test/ui-mocks');
  return { Input: passthrough('input') };
});
vi.mock('@/shared/components/ui/button', async () => {
  const { passthrough } = await import('@/test/ui-mocks');
  return { Button: passthrough('button') };
});
vi.mock('@/features/complex/hooks/useSelectedComplex', () => ({
  useSelectedComplex: vi.fn().mockReturnValue({ selectedComplexId: 'c1' }),
}));
vi.mock('@/features/clients/hooks/useClients', () => ({
  useClients: vi.fn().mockReturnValue({
    data: { pages: [{ clients: [] }] },
    isLoading: false,
    hasNextPage: false,
    fetchNextPage: vi.fn(),
    isFetchingNextPage: false,
  }),
}));
vi.mock('@/features/clients/hooks/useUpdateClient', () => ({
  useUpdateClient: () => ({ mutate: vi.fn(), isPending: false }),
}));
vi.mock('@/features/clients/components/ClientGrid', () => ({ ClientGrid: () => <div>Grid</div> }));
vi.mock('@/features/clients/components/ClientDetail', () => ({ ClientDetail: () => null }));
vi.mock('@/features/clients/components/BlockClientModal', () => ({ BlockClientModal: () => null }));
vi.mock('sonner', () => ({ toast: { success: vi.fn() } }));

import { useSelectedComplex } from '@/features/complex';

// `useClientsPage` reads/writes the search term through `useSearchParams`
// (see M2), which throws outside a Router; `useClientActions`'s own
// `useClient` needs a `QueryClient` even while disabled.
function renderClientsPage(ClientsPage: ComponentType, initialEntries: string[] = ['/clients']) {
  return renderWithProviders(<ClientsPage />, { initialEntries });
}

describe('ClientsPage', () => {
  it('renders page title', async () => {
    const ClientsPage = (await import('./ClientsPage')).default;
    renderClientsPage(ClientsPage);
    expect(screen.getByText('Clientes')).toBeInTheDocument();
  });

  it('shows empty state when no clients', async () => {
    const ClientsPage = (await import('./ClientsPage')).default;
    renderClientsPage(ClientsPage);
    expect(screen.getByTestId('empty')).toBeInTheDocument();
  });

  it('returns null when no selectedComplexId', async () => {
    vi.mocked(useSelectedComplex).mockReturnValue({
      selectedComplexId: null,
      complex: null,
      complexes: [],
      needsOnboarding: false,
      isLoading: false,
      isFetching: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    });
    const ClientsPage = (await import('./ClientsPage')).default;
    const { container } = renderClientsPage(ClientsPage);
    expect(container.innerHTML).toBe('');
  });

  it('opens with the search term named by ?q=, surviving a refresh', async () => {
    // The previous test leaves `useSelectedComplex` mocked to `null` — reset
    // it explicitly instead of relying on file order for a real complex id.
    vi.mocked(useSelectedComplex).mockReturnValue({
      selectedComplexId: 'c1',
      complex: null,
      complexes: [],
      needsOnboarding: false,
      isLoading: false,
      isFetching: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    });
    const ClientsPage = (await import('./ClientsPage')).default;
    renderClientsPage(ClientsPage, ['/clients?q=juan']);
    expect(screen.getByDisplayValue('juan')).toBeInTheDocument();
  });
});
