import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { createMemoryRouter, Outlet, RouterProvider, useParams } from 'react-router-dom';
import { publicRoutes } from './publicRoutes';

vi.mock('@/shared/components/layout/PublicLayout', () => ({
  PublicLayout: () => <Outlet />,
}));

// The booking routes are stubbed: these tests are about where a URL lands,
// not about what each page shows once it is there.
vi.mock('./booking/ComplexPageRoute', () => ({
  ComplexPageRoute: function ComplexPageStub() {
    const { slug } = useParams<{ slug: string }>();
    return <div>complex page {slug}</div>;
  },
}));
vi.mock('./booking/BookPageRoute', () => ({ BookPageRoute: () => <div>book page</div> }));
vi.mock('./booking/BookConfirmRoute', () => ({ BookConfirmRoute: () => <div>confirm page</div> }));
vi.mock('./booking/BookSuccessRoute', () => ({ BookSuccessRoute: () => <div>success page</div> }));
vi.mock('./booking/BookCancelRoute', () => ({ BookCancelRoute: () => <div>cancel page</div> }));

function renderAt(url: string) {
  const router = createMemoryRouter(publicRoutes, { initialEntries: [url] });
  render(<RouterProvider router={router} />);
  return router;
}

describe('public complex routes', () => {
  it('renders the complex page under /c/:slug', async () => {
    renderAt('/c/los-alamos');

    expect(await screen.findByText('complex page los-alamos')).toBeInTheDocument();
  });

  it('does not treat /c/book as a legacy booking URL: a complex whose slug is "book" keeps its own page', async () => {
    const router = renderAt('/c/book');

    expect(await screen.findByText('complex page book')).toBeInTheDocument();
    expect(router.state.location.pathname).toBe('/c/book');
  });

  it.each([
    ['/c/los-alamos/book', 'book page'],
    ['/c/los-alamos/book/confirm', 'confirm page'],
    ['/c/los-alamos/book/success', 'success page'],
    ['/c/los-alamos/book/cancel', 'cancel page'],
  ])('renders %s under the /c/ prefix', async (url, page) => {
    const router = renderAt(url);

    expect(await screen.findByText(page)).toBeInTheDocument();
    expect(router.state.location.pathname).toBe(url);
  });
});

describe('legacy complex URLs', () => {
  it('renders the complex page at the legacy /:slug URL by redirecting to /c/:slug', async () => {
    const router = renderAt('/los-alamos');

    expect(await screen.findByText('complex page los-alamos')).toBeInTheDocument();
    expect(router.state.location.pathname).toBe('/c/los-alamos');
  });

  it('keeps the query string and hash when it redirects the legacy URL', async () => {
    const router = renderAt('/los-alamos?date=2099-01-05&time=08%3A30#turnos');

    await screen.findByText('complex page los-alamos');
    expect(router.state.location.pathname).toBe('/c/los-alamos');
    expect(router.state.location.search).toBe('?date=2099-01-05&time=08%3A30');
    expect(router.state.location.hash).toBe('#turnos');
  });

  it('replaces the legacy entry instead of pushing it, so Back does not return to the old URL', async () => {
    const router = renderAt('/los-alamos');

    await screen.findByText('complex page los-alamos');
    expect(router.state.historyAction).toBe('REPLACE');
  });

  // Booking links already sent (confirmation emails, WhatsApp messages, the
  // MercadoPago back_urls on in-flight preferences) carry these old paths.
  it.each([
    ['/los-alamos/book', '/c/los-alamos/book', 'book page'],
    ['/los-alamos/book/confirm', '/c/los-alamos/book/confirm', 'confirm page'],
    ['/los-alamos/book/success', '/c/los-alamos/book/success', 'success page'],
    ['/los-alamos/book/cancel', '/c/los-alamos/book/cancel', 'cancel page'],
  ])('redirects the legacy %s to %s', async (legacyUrl, newPath, page) => {
    const router = renderAt(legacyUrl);

    expect(await screen.findByText(page)).toBeInTheDocument();
    expect(router.state.location.pathname).toBe(newPath);
  });

  it('keeps the MercadoPago failure query on the redirected booking page', async () => {
    const router = renderAt('/los-alamos/book?error=payment_failed');

    await screen.findByText('book page');
    expect(router.state.location.pathname).toBe('/c/los-alamos/book');
    expect(router.state.location.search).toBe('?error=payment_failed');
  });

  it('keeps the booking token and status on the redirected success page', async () => {
    const router = renderAt('/los-alamos/book/success?token=t1&status=pending');

    await screen.findByText('success page');
    expect(router.state.location.pathname).toBe('/c/los-alamos/book/success');
    expect(router.state.location.search).toBe('?token=t1&status=pending');
  });

  it('keeps the cancellation token on the redirected cancel page', async () => {
    const router = renderAt('/los-alamos/book/cancel?token=abc');

    await screen.findByText('cancel page');
    expect(router.state.location.pathname).toBe('/c/los-alamos/book/cancel');
    expect(router.state.location.search).toBe('?token=abc');
  });
});
