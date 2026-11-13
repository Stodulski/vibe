import { describe, it, expect, vi, beforeEach } from 'vitest';
import userEvent from '@testing-library/user-event';
import { renderWithProviders, screen, waitFor } from '@/test/test-utils';
import { ComplexForm } from '../ComplexForm';
import { makeComplex } from '@/test/factories';

const mockMutate = vi.fn();

vi.mock('../../hooks/useUpdateComplex', () => ({
  useUpdateComplex: () => ({ mutate: mockMutate, isPending: false }),
}));
vi.mock('../../hooks/useCreateComplex', () => ({
  useCreateComplex: () => ({ mutate: vi.fn(), isPending: false }),
}));

describe('amenities, newer options', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('offers the newer services under their labels, and the relabelled ones', async () => {
    const user = userEvent.setup();
    renderWithProviders(<ComplexForm complex={makeComplex({ amenities: [] })} />);

    for (const name of [
      'Cumpleaños',
      'Parrilla',
      'Quincho',
      'Ayuda médica',
      'Escuelita deportiva',
      'Kiosco o buffet',
      'Juegos para chicos',
      'Alquiler de equipo',
      'Venta de pelotas y accesorios',
    ]) {
      expect(screen.getByRole('checkbox', { name: name })).toBeInTheDocument();
    }

    await user.click(screen.getByRole('checkbox', { name: 'Quincho' }));
    await user.click(screen.getByRole('button', { name: /Guardar cambios/ }));
    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalledTimes(1);
    });
    expect((mockMutate.mock.calls[0]?.[0] as { amenities: string[] }).amenities).toEqual(['quincho']);
  });
});
