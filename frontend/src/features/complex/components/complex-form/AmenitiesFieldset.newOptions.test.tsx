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

  it('saves a newer amenity ticked under its label', async () => {
    const user = userEvent.setup();
    renderWithProviders(<ComplexForm complex={makeComplex({ amenities: [] })} />);

    await user.click(screen.getByRole('checkbox', { name: 'Quincho' }));
    await user.click(screen.getByRole('button', { name: /Guardar cambios/ }));
    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalledTimes(1);
    });
    expect((mockMutate.mock.calls[0]?.[0] as { amenities: string[] }).amenities).toEqual(['quincho']);
  });
});
