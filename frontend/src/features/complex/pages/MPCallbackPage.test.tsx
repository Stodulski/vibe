import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { createWrapper } from '@/test/test-utils';

vi.mock('@/shared/hooks/usePageTitle', () => ({ usePageTitle: vi.fn() }));
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
    // Without code/state params, component immediately goes to error state,
    // and stays there with a way to start over instead of bouncing away.
    expect(screen.getByText(/el enlace de conexión venció/i)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /volver a intentar/i })).toHaveAttribute('href', '/settings');
  });

  it('renders AppHeader', async () => {
    const Page = (await import('./MPCallbackPage')).default;
    render(<Page />, { wrapper: createWrapper(['/settings/mp/callback']) });
    expect(screen.getByTestId('app-header')).toBeInTheDocument();
  });
});
