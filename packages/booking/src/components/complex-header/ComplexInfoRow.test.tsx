import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ComplexInfoRow } from './ComplexInfoRow';
import type { PublicComplex } from '../../api/types';

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

  it('shows the province after the city', () => {
    render(
      <ComplexInfoRow
        complex={makeComplex({ address: 'Piquiyin', city: 'Caleta Olivia', province: 'Santa Cruz' })}
      />,
    );
    expect(screen.getByText('Piquiyin, Caleta Olivia, Santa Cruz')).toBeInTheDocument();
  });

  it('makes the phone callable', () => {
    render(<ComplexInfoRow complex={makeComplex()} />);
    expect(screen.getByRole('link', { name: /\+541155550000/ })).toHaveAttribute('href', 'tel:+541155550000');
  });

  it('offers WhatsApp to the club in a new tab, next to the phone', () => {
    render(<ComplexInfoRow complex={makeComplex()} />);
    const link = screen.getByRole('link', { name: 'Escribir por WhatsApp a Club Test' });
    expect(link).toHaveAttribute('href', 'https://wa.me/541155550000');
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it('links the club\'s Instagram profile in a new tab', () => {
    render(<ComplexInfoRow complex={makeComplex({ instagram: 'club.test' })} />);
    const link = screen.getByRole('link', { name: 'Instagram de Club Test: @club.test' });
    expect(link).toHaveAttribute('href', 'https://www.instagram.com/club.test/');
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
    expect(link).toHaveTextContent('@club.test');
  });

  it('shows no Instagram row when the club has none', () => {
    render(<ComplexInfoRow complex={makeComplex()} />);
    expect(screen.queryByRole('link', { name: /Instagram/ })).not.toBeInTheDocument();
  });

  it('offers no WhatsApp link when the club has no phone', () => {
    render(<ComplexInfoRow complex={makeComplex({ phone: '' })} />);
    expect(screen.queryByRole('link', { name: /WhatsApp/ })).not.toBeInTheDocument();
  });
});
