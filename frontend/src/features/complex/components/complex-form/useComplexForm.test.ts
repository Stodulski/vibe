import { renderHook, act } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { useComplexForm } from './useComplexForm';
import type { Complex } from '@/shared/types/api.types';

const mockCreateMutate = vi.fn();
const mockUpdateMutate = vi.fn();

vi.mock('../../hooks/useCreateComplex', () => ({
  useCreateComplex: () => ({ mutate: mockCreateMutate, isPending: false }),
}));

vi.mock('../../hooks/useUpdateComplex', () => ({
  useUpdateComplex: () => ({ mutate: mockUpdateMutate, isPending: false }),
}));

const baseValues = {
  name: 'Club Test',
  slug: 'club-test',
  formatted_address: 'Av. Test 123, CABA, Buenos Aires',
  address: 'Av. Test 123',
  city: 'CABA',
  province: 'Buenos Aires',
  latitude: -34.6,
  longitude: -58.4,
  phone: '1155550000',
  email: '',
  deposit_percentage: 30,
  cancellation_hours: 24,
  amenities: [],
};

describe('useComplexForm', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // Used to run one render later, in a `useEffect` watching `watch('name')` —
  // there was a frame where the name had changed but the slug had not.
  // Deriving it in the name field's own `onChange` (see `onNameChange`) keeps
  // them in the same update, and stops once the slug has been edited by hand.
  it('derives the slug from the name until the slug is edited by hand', () => {
    const { result } = renderHook(() => useComplexForm({}));

    act(() => {
      result.current.onNameChange('Mi Club Padel');
    });
    expect(result.current.form.getValues('slug')).toBe('mi-club-padel');

    act(() => {
      result.current.onSlugManualEdit();
    });
    act(() => {
      result.current.onNameChange('Otro Nombre');
    });
    expect(result.current.form.getValues('slug')).toBe('mi-club-padel');
  });

  // The old `mutation = isEdit ? updateMutation : createMutation` union
  // needed a `@ts-expect-error` to call `.mutate` at all, because create and
  // update hit differently-typed endpoints. That suppression would have
  // stayed silent through a real payload mismatch; branching properly means
  // each call is checked against its own mutation.
  it('sends slug on both create and update — the public URL is editable in both modes', () => {
    const { result: createResult } = renderHook(() => useComplexForm({}));
    act(() => {
      createResult.current.onSubmit(baseValues);
    });

    expect(mockCreateMutate).toHaveBeenCalledTimes(1);
    const createPayload = mockCreateMutate.mock.calls[0]?.[0] as Record<string, unknown>;
    expect(createPayload.slug).toBe('club-test');

    const complex = { id: 'c1', ...baseValues } as unknown as Complex;
    const { result: editResult } = renderHook(() => useComplexForm({ complex }));
    act(() => {
      editResult.current.onSubmit({ ...baseValues, slug: 'club-test-renamed' });
    });

    expect(mockUpdateMutate).toHaveBeenCalledTimes(1);
    const updatePayload = mockUpdateMutate.mock.calls[0]?.[0] as Record<string, unknown>;
    expect(updatePayload.slug).toBe('club-test-renamed');
  });
});
