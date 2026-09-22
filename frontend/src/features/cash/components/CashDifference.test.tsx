import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { CashDifference } from './CashDifference';

describe('CashDifference', () => {
  it('labels a positive difference as sobrante', () => {
    render(<CashDifference difference={5000} />);
    expect(screen.getByText(/Sobrante/)).toBeInTheDocument();
  });

  it('labels a negative difference as faltante, showing the absolute amount', () => {
    render(<CashDifference difference={-5000} />);
    expect(screen.getByText(/Faltante/)).toBeInTheDocument();
    expect(screen.getByText(/\$50\b/)).toBeInTheDocument();
  });

  it('labels a zero difference as sin diferencia, with no amount', () => {
    render(<CashDifference difference={0} />);
    expect(screen.getByText('Sin diferencia')).toBeInTheDocument();
  });
});
