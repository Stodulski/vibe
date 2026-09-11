import { vi } from 'vitest';

export const mockCreateMutate = vi.fn();
export const mockUpdateMutate = vi.fn();

vi.mock('./AddressInput', () => ({
  AddressInput: (props: { id: string; value: string; onChange: (v: string) => void; placeholder?: string }) => (
    <input
      id={props.id}
      value={props.value}
      onChange={(e) => {
        props.onChange(e.target.value);
      }}
      placeholder={props.placeholder}
      data-testid="address-input"
    />
  ),
}));

vi.mock('../hooks/useCreateComplex', () => ({
  useCreateComplex: () => ({
    mutate: mockCreateMutate,
    isPending: false,
  }),
}));

vi.mock('../hooks/useUpdateComplex', () => ({
  useUpdateComplex: () => ({
    mutate: mockUpdateMutate,
    isPending: false,
  }),
}));
