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

describe('amenities, as a field of the complex form', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('ticks what the complex already lists', () => {
    renderWithProviders(<ComplexForm complex={makeComplex({ amenities: ['bar', 'wifi'] })} />);
    expect(screen.getByRole('checkbox', { name: /Bar/ })).toBeChecked();
    expect(screen.getByRole('checkbox', { name: /Wi-Fi/ })).toBeChecked();
    expect(screen.getByRole('checkbox', { name: /Estacionamiento/ })).not.toBeChecked();
  });

  it('saves the amenities together with the rest of the profile, on one submit', async () => {
    const user = userEvent.setup();
    renderWithProviders(<ComplexForm complex={makeComplex({ name: 'Club Norte', amenities: [] })} />);

    await user.click(screen.getByRole('checkbox', { name: /Wi-Fi/ }));
    await user.click(screen.getByRole('checkbox', { name: /Estacionamiento/ }));
    await user.click(screen.getByRole('button', { name: /Guardar cambios/ }));

    // One button, one request, carrying both the written fields and the
    // services — the whole point of folding them into this form.
    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalledTimes(1);
    });
    const sent = mockMutate.mock.calls[0]?.[0] as { name: string; amenities: string[] };
    expect(sent.name).toBe('Club Norte');
    // Canonical order, not the order the boxes were ticked.
    expect(sent.amenities).toEqual(['parking', 'wifi']);
  });

  it('unticking removes it from what is sent', async () => {
    const user = userEvent.setup();
    renderWithProviders(<ComplexForm complex={makeComplex({ amenities: ['parking', 'bar'] })} />);

    await user.click(screen.getByRole('checkbox', { name: /Bar/ }));
    await user.click(screen.getByRole('button', { name: /Guardar cambios/ }));

    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalledTimes(1);
    });
    const sent = mockMutate.mock.calls[0]?.[0] as { amenities: string[] };
    expect(sent.amenities).toEqual(['parking']);
  });

  it('renders a complex cached from before the field existed', () => {
    // A response already in the React Query cache has no `amenities` key. That
    // undefined reaching the checkbox group took the settings page down.
    const stale = { ...makeComplex(), amenities: undefined as unknown as [] };
    renderWithProviders(<ComplexForm complex={stale} />);
    expect(screen.getByRole('checkbox', { name: /Bar/ })).not.toBeChecked();
  });
});
