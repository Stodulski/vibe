import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ComplexInfoCard } from './ComplexInfoCard';
import { makeComplex } from '@/test/factories';

describe('ComplexInfoCard', () => {
  it('shows the public address under the /c/ prefix', () => {
    render(<ComplexInfoCard complex={makeComplex({ slug: 'los-alamos' })} />);

    expect(screen.getByText('/c/los-alamos')).toBeInTheDocument();
  });

  it('opens the public page under the /c/ prefix', () => {
    render(<ComplexInfoCard complex={makeComplex({ slug: 'los-alamos' })} />);

    expect(screen.getByRole('link')).toHaveAttribute('href', '/c/los-alamos');
  });
});
