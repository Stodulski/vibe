import { beforeAll, afterAll, describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { RouteErrorPage } from './RouteErrorPage';

// Suppress the expected "Layout render error" console noise React logs for
// caught render errors, same pattern as ErrorBoundary's own test helpers.
const originalError = console.error;
beforeAll(() => {
  console.error = (...args: unknown[]) => {
    if (typeof args[0] === 'string' && args[0].includes('Layout render error')) {
      return;
    }
    originalError.call(console, ...args);
  };
});
afterAll(() => {
  console.error = originalError;
});

function ThrowingLayout(): never {
  throw new Error('Layout render error');
}

describe('RouteErrorPage', () => {
  it('renders the fallback UI when a parent layout throws during render', () => {
    // Mirrors router.tsx: `errorElement` sits on the route wrapping a layout
    // (here `ThrowingLayout`, standing in for DashboardLayout/AdminLayout/
    // PublicLayout/ProtectedRoute/etc.), not just the lazy page inside it —
    // this fails if the errorElement were placed only around a leaf page,
    // the way `routeHelpers.tsx`'s per-page `<ErrorBoundary>` is.
    const router = createMemoryRouter(
      [
        {
          element: <ThrowingLayout />,
          errorElement: <RouteErrorPage />,
          children: [{ path: '/', element: <div>Page content</div> }],
        },
      ],
      { initialEntries: ['/'] },
    );

    render(<RouterProvider router={router} />);

    expect(screen.getByText(/algo sali.? mal/i)).toBeInTheDocument();
    expect(screen.getByRole('alert')).toBeInTheDocument();
    expect(screen.queryByText('Page content')).not.toBeInTheDocument();
  });

  it('offers retry and back-home actions', () => {
    const router = createMemoryRouter(
      [
        {
          element: <ThrowingLayout />,
          errorElement: <RouteErrorPage />,
          children: [{ path: '/', element: <div>Page content</div> }],
        },
      ],
      { initialEntries: ['/'] },
    );

    render(<RouterProvider router={router} />);

    expect(screen.getByRole('button', { name: /reintentar/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /volver al inicio/i })).toBeInTheDocument();
  });
});
