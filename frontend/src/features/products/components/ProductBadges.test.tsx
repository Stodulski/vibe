import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ProductBadges } from './ProductBadges';
import { makeProduct } from '@/test/factories';

describe('ProductBadges', () => {
  it('shows nothing extra for an active, well-stocked product', () => {
    render(<ProductBadges product={makeProduct({ active: true, low_stock: false, needs_stock_review: false })} />);
    expect(screen.queryByText('Inactivo')).not.toBeInTheDocument();
    expect(screen.queryByText('Stock bajo')).not.toBeInTheDocument();
    expect(screen.queryByText('Revisar stock')).not.toBeInTheDocument();
  });

  it('shows the inactive badge for a deactivated product', () => {
    render(<ProductBadges product={makeProduct({ active: false })} />);
    expect(screen.getByText('Inactivo')).toBeInTheDocument();
  });

  it('shows the low-stock badge', () => {
    render(<ProductBadges product={makeProduct({ low_stock: true, needs_stock_review: false })} />);
    expect(screen.getByText('Stock bajo')).toBeInTheDocument();
  });

  it('prefers "Revisar stock" over "Stock bajo" when both are computed true (negative stock is also at-or-below any threshold)', () => {
    render(<ProductBadges product={makeProduct({ low_stock: true, needs_stock_review: true })} />);
    expect(screen.getByText('Revisar stock')).toBeInTheDocument();
    expect(screen.queryByText('Stock bajo')).not.toBeInTheDocument();
  });
});
