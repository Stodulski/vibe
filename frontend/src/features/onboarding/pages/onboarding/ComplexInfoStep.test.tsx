import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ComplexInfoStep } from './ComplexInfoStep';
import { makeComplex } from '@/test/factories';
import type { Complex } from '@/shared/types/api.types';

vi.mock('@/features/complex/components/ComplexForm', () => ({
  ComplexForm: ({ complex }: { complex?: Complex }) => (
    <div data-testid="complex-form">{complex ? 'Editing' : 'Creating'}</div>
  ),
}));

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
});
