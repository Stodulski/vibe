import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AppShellLoader } from './AppShellLoader';
import { routeSkeleton } from '@/app/router/routeSkeletons';

function renderAt(pathname: string) {
  return render(
    <MemoryRouter initialEntries={[pathname]}>
      <AppShellLoader />
    </MemoryRouter>,
  );
}

function mainHtml(container: HTMLElement) {
  return container.querySelector('main .mx-auto')?.innerHTML;
}

describe('AppShellLoader', () => {
  it('announces loading once and draws the sidebar rail with one bar per nav item', () => {
    const { container } = renderAt('/dashboard');
    expect(screen.getAllByRole('status')).toHaveLength(1);
    const rail = container.querySelector('.bg-bg-sidebar');
    expect(rail).not.toBeNull();
    expect(rail?.querySelectorAll('.space-y-0\\.5 > div')).toHaveLength(7);
  });

  it.each(['/cash', '/dashboard', '/courts', '/clients', '/reports'] as const)(
    'draws the %s route fallback inside <main>',
    (pathname) => {
      const { container } = renderAt(pathname);
      const expected = render(<>{routeSkeleton(pathname)}</>).container.innerHTML;
      expect(mainHtml(container)).toBe(expected);
    },
  );

  it('keeps the cash fallback specific: it carries the cash section tabs', () => {
    const { container } = renderAt('/cash');
    expect(container.querySelectorAll('main .border-b .h-11')).toHaveLength(3);
  });

  it('falls back to the neutral content for an unknown path', () => {
    const { container } = renderAt('/something-else');
    expect(container.querySelector('main .h-56')).not.toBeNull();
    expect(container.querySelector('main .h-24')).not.toBeNull();
  });
});
