import { render, screen } from '@testing-library/react';
import { createWrapper } from '@/test/test-utils';

vi.mock('@/shared/hooks/usePageTitle', () => ({ usePageTitle: vi.fn() }));
vi.mock('@/shared/stores', () => ({
  useStore: (selector: (s: { setSelectedComplexId: () => void }) => unknown) =>
    selector({ setSelectedComplexId: vi.fn() }),
}));
vi.mock('@/shared/components/layout/AppHeader', () => ({
  AppHeader: () => <header data-testid="app-header">Header</header>,
}));
vi.mock('@/shared/components/common/LoadingSpinner', () => ({
  LoadingSpinner: () => <div role="status">Loading</div>,
}));

describe('MPCallbackPage', () => {
  it('shows error state when no code param is present', async () => {
    const Page = (await import('./MPCallbackPage')).default;
    render(<Page />, { wrapper: createWrapper(['/settings/mp/callback']) });
    // Without code/state params, component immediately goes to error state
    expect(screen.getByText(/error al conectar/i)).toBeInTheDocument();
  });

  it('renders AppHeader', async () => {
    const Page = (await import('./MPCallbackPage')).default;
    render(<Page />, { wrapper: createWrapper(['/settings/mp/callback']) });
    expect(screen.getByTestId('app-header')).toBeInTheDocument();
  });
});
