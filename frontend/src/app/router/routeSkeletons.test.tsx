import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import type { ReactNode } from 'react';
import { routeSkeleton, skeletonForPath } from './routeSkeletons';

function html(node: ReactNode) {
  return render(<>{node}</>).container.innerHTML;
}

const CASES = [
  ['/dashboard', '/dashboard'],
  ['/bookings', '/bookings'],
  ['/cash', '/cash'],
  ['/cash/sell', '/cash/sell'],
  ['/cash/sessions/s-1', '/cash/sessions/:sessionId'],
  ['/cash/products', '/cash/products'],
  ['/cash/products/p-9', '/cash/products/:productId'],
  ['/courts', '/courts'],
  ['/clients', '/clients'],
  ['/reports', '/reports'],
  ['/settings', '/settings'],
  ['/profile', '/profile'],
] as const;

describe('skeletonForPath', () => {
  it.each(CASES)('%s resolves to the skeleton its route uses as fallback', (pathname, pattern) => {
    const found = skeletonForPath(pathname);
    expect(found).not.toBeNull();
    expect(html(found)).toBe(html(routeSkeleton(pattern)));
  });

  it('gives different content to different routes', () => {
    const rendered = new Set(CASES.map(([, pattern]) => html(routeSkeleton(pattern))));
    expect(rendered.size).toBe(CASES.length);
  });

  it('tolerates a trailing slash', () => {
    expect(html(skeletonForPath('/cash/'))).toBe(html(routeSkeleton('/cash')));
  });

  it.each(['/', '/nope', '/cash/unknown', '/cash/sessions/a/b'])('%s has no skeleton', (pathname) => {
    expect(skeletonForPath(pathname)).toBeNull();
  });
});
