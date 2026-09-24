import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { CashSectionTabs } from './CashSectionTabs';

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <CashSectionTabs />
    </MemoryRouter>,
  );
}

describe('CashSectionTabs', () => {
  it('renders links (not buttons) for all three tabs, deep-linkable and back-button friendly', () => {
    renderAt('/cash');
    expect(screen.getByRole('link', { name: 'Turno' })).toHaveAttribute('href', '/cash');
    expect(screen.getByRole('link', { name: 'Vender' })).toHaveAttribute('href', '/cash/sell');
    expect(screen.getByRole('link', { name: 'Productos' })).toHaveAttribute('href', '/cash/products');
  });

  it('marks only Turno active on the exact /cash route', () => {
    renderAt('/cash');
    expect(screen.getByRole('link', { name: 'Turno' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('link', { name: 'Productos' })).not.toHaveAttribute('aria-current');
  });

  it('marks Productos active on /cash/products', () => {
    renderAt('/cash/products');
    expect(screen.getByRole('link', { name: 'Productos' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('link', { name: 'Turno' })).not.toHaveAttribute('aria-current');
  });

  it('marks Vender active on /cash/sell', () => {
    renderAt('/cash/sell');
    expect(screen.getByRole('link', { name: 'Vender' })).toHaveAttribute('aria-current', 'page');
  });
});
