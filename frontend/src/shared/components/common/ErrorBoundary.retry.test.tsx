import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import './error-boundary-test-helpers';
import { ErrorBoundary } from './ErrorBoundary';
import { ThrowingComponent } from './error-boundary-test-helpers';

// The facade, not the SDK: reporting goes through
// `shared/lib/observability`, which queues until `@sentry/react` has
// finished loading off the critical path.
vi.mock('@/shared/lib/observability', () => ({
  captureException: vi.fn(),
}));

describe('ErrorBoundary retry and reporting', () => {
  it('resets error state when retry is clicked', async () => {
    const user = userEvent.setup();

    // Use a ref-like pattern to control throwing
    let shouldThrow = true;
    function ControlledComponent() {
      if (shouldThrow) throw new Error('Test error');
      return <div>Content is fine</div>;
    }

    render(
      <ErrorBoundary>
        <ControlledComponent />
      </ErrorBoundary>,
    );

    expect(screen.getByText(/algo sali.? mal/i)).toBeInTheDocument();

    // Stop throwing before clicking retry
    shouldThrow = false;
    await user.click(screen.getByRole('button', { name: /reintentar/i }));

    expect(screen.getByText('Content is fine')).toBeInTheDocument();
  });

  it('calls Sentry.captureException on error', async () => {
    const { captureException } = await import('@/shared/lib/observability');

    render(
      <ErrorBoundary>
        <ThrowingComponent shouldThrow={true} />
      </ErrorBoundary>,
    );

    expect(captureException).toHaveBeenCalledWith(
      expect.any(Error),
      expect.objectContaining({
        extra: expect.objectContaining({
          componentStack: expect.any(String) as unknown as string,
        }) as unknown as Record<string, unknown>,
      }),
    );
  });
});
