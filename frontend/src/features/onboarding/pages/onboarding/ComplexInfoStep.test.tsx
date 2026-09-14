import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ComplexInfoStep } from './ComplexInfoStep';
import { makeComplex } from '@/test/factories';
import type { ReactNode } from 'react';
import type { Complex } from '@/shared/types/api.types';

vi.mock('@/features/complex', async () => {
  const actual = await vi.importActual<typeof import('@/features/complex')>('@/features/complex');
  return {
    ...actual,
    ComplexForm: ({ complex, fields }: { complex?: Complex; fields?: (props: unknown) => ReactNode }) => (
      <div data-testid="complex-form">
        {complex ? 'Editing' : 'Creating'}
        {fields?.({ form: { control: {} } })}
      </div>
    ),
    IdentityGroup: () => <div data-testid="identity-group" />,
    LocationGroup: () => <div data-testid="location-group" />,
    AmenitiesGroup: () => <div data-testid="amenities-group" />,
  };
});

describe('ComplexInfoStep', () => {
  it('renders ComplexForm for new complex', () => {
    render(<ComplexInfoStep currentComplex={null} onComplexCreated={vi.fn()} />);
    expect(screen.getByTestId('complex-form')).toHaveTextContent('Creating');
  });

  it('renders ComplexForm for existing complex', () => {
    const complex = makeComplex({ id: 'c1', name: 'Test' });
    render(<ComplexInfoStep currentComplex={complex} onComplexCreated={vi.fn()} />);
    expect(screen.getByTestId('complex-form')).toHaveTextContent('Editing');
  });

  it('asks for identity, location and amenities, in that order', () => {
    render(<ComplexInfoStep currentComplex={null} onComplexCreated={vi.fn()} />);
    const identity = screen.getByTestId('identity-group');
    const location = screen.getByTestId('location-group');
    const amenities = screen.getByTestId('amenities-group');
    expect(identity.compareDocumentPosition(location) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(location.compareDocumentPosition(amenities) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });
});
