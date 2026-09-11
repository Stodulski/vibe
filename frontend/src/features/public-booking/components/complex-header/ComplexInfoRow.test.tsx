import { render, screen } from '@testing-library/react';
import { ComplexInfoRow } from './ComplexInfoRow';
import type { PublicComplex } from '@/shared/types/api.types';

function makeComplex(overrides: Partial<PublicComplex> = {}): PublicComplex {
  return {
    id: 'c1',
    name: 'Club Test',
    slug: 'club-test',
    amenities: [],
    address: 'Av. Libertador 1234',
    city: 'Buenos Aires',
    province: 'Buenos Aires',
    country_code: 'AR',
    currency: 'ARS',
    phone: '+541155550000',
    deposit_percentage: 30,
    cancellation_hours: 24,
    payments_enabled: true,
    ...overrides,
  };
}

describe('ComplexInfoRow', () => {
  // This used to assert a `title` attribute carrying the address that the
  // visible text truncated away. A hover is not available on the phones most
  // of this page's visitors use, so the address is shown in full instead.
  it('shows the whole address', () => {
    render(<ComplexInfoRow complex={makeComplex()} />);
    expect(screen.getByText('Av. Libertador 1234, Buenos Aires')).toBeInTheDocument();
  });

  it('makes the phone callable', () => {
    render(<ComplexInfoRow complex={makeComplex()} />);
    expect(screen.getByRole('link', { name: /\+541155550000/ })).toHaveAttribute('href', 'tel:+541155550000');
  });
});
