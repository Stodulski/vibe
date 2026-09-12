import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { ScrollToTop } from './ScrollToTop';
import { getScrollRestorationKey } from './scrollRestorationKey';

// U-02: every choice in the public booking flow (`useComplexPageState`) is a
// `setSearchParams` call on the same route, which React Router treats as a
// new location by default — `<ScrollRestoration />`'s default key scrolled
// the page back to the top on every one of those taps. The fix keys the
// restoration on the pathname alone, so a query-only change is not a new key.
describe('getScrollRestorationKey — U-02', () => {
  it('is the same key for two locations that differ only in their query string', () => {
    const withoutQuery = getScrollRestorationKey({ pathname: '/los-alamos' });
    const withQuery = getScrollRestorationKey({ pathname: '/los-alamos' });

    expect(withQuery).toBe(withoutQuery);
  });

  it('drops the search string entirely, so a day/duration/hour tap keys the same as the bare route', () => {
    // `ScrollRestoration`'s `getKey` receives a `Location`, whose `pathname`
    // never carries the query — this pins the one property the fix reads.
    expect(getScrollRestorationKey({ pathname: '/los-alamos' })).toBe('/los-alamos');
  });

  it('differs for two different pathnames, so a real route change still gets its own key', () => {
    const confirm = getScrollRestorationKey({ pathname: '/los-alamos/book/confirm' });
    const storefront = getScrollRestorationKey({ pathname: '/los-alamos' });

    expect(confirm).not.toBe(storefront);
  });
});

describe('ScrollToTop', () => {
  it('renders the matched route beneath the scroll restoration', () => {
    const router = createMemoryRouter(
      [
        {
          element: <ScrollToTop />,
          children: [{ path: '/los-alamos', element: <div>Storefront</div> }],
        },
      ],
      { initialEntries: ['/los-alamos'] },
    );

    render(<RouterProvider router={router} />);

    expect(screen.getByText('Storefront')).toBeInTheDocument();
  });
});
