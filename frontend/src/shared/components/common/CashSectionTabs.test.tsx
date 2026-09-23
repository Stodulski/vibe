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
    expect(screen.getByRole('link', { name: 'Turno' })).toHaveClass('border-primary-500');
    expect(screen.getByRole('link', { name: 'Productos' })).not.toHaveClass('border-primary-500');
  });

  it('marks Productos active on /cash/products', () => {
    renderAt('/cash/products');
    expect(screen.getByRole('link', { name: 'Productos' })).toHaveClass('border-primary-500');
    expect(screen.getByRole('link', { name: 'Turno' })).not.toHaveClass('border-primary-500');
  });

  it('marks Vender active on /cash/sell', () => {
    renderAt('/cash/sell');
    expect(screen.getByRole('link', { name: 'Vender' })).toHaveClass('border-primary-500');
  });
});
