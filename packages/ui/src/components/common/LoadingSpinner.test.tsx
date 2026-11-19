import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { LoadingSpinner } from './LoadingSpinner';

describe('LoadingSpinner', () => {
  it('shows custom label', () => {
    render(<LoadingSpinner label="Procesando pago..." />);
    expect(screen.getByRole('status')).toHaveAttribute('aria-label', 'Procesando pago...');
  });
});
