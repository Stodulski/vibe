import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { SidebarNav } from './SidebarNav';

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <SidebarNav collapsed={false} isMobile={false} onPrefetch={() => () => undefined} />
    </MemoryRouter>,
  );
}

describe('SidebarNav — active path resolution', () => {
  it('marks Caja active on the exact /cash route', () => {
    renderAt('/cash');
    expect(screen.getByRole('link', { name: 'Caja' })).toHaveAttribute('aria-current', 'page');
  });

  it('marks Caja active on /cash/sell (T5a: no new sidebar item for the Vender tab)', () => {
    renderAt('/cash/sell');
    expect(screen.getByRole('link', { name: 'Caja' })).toHaveAttribute('aria-current', 'page');
  });

  it('marks Caja active on /cash/products (T5a: no new sidebar item for the Productos tab)', () => {
    renderAt('/cash/products');
    expect(screen.getByRole('link', { name: 'Caja' })).toHaveAttribute('aria-current', 'page');
  });

  it('marks Caja active on a product detail route (/cash/products/:id)', () => {
    renderAt('/cash/products/p1');
    expect(screen.getByRole('link', { name: 'Caja' })).toHaveAttribute('aria-current', 'page');
  });

  it('does not mark Caja active on an unrelated route', () => {
    renderAt('/dashboard');
    expect(screen.getByRole('link', { name: 'Caja' })).not.toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('link', { name: 'Dashboard' })).toHaveAttribute('aria-current', 'page');
  });
});
