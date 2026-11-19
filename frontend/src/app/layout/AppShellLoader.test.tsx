import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
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
  it.each(['/cash', '/dashboard', '/courts', '/clients', '/reports', '/events', '/kit'] as const)(
    'draws the %s route fallback inside <main>',
    (pathname) => {
      const { container } = renderAt(pathname);
      const expected = render(<>{routeSkeleton(pathname)}</>).container.innerHTML;
      expect(mainHtml(container)).toBe(expected);
    },
  );
});
